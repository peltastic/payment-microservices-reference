import {
  InjectQueue,
  OnWorkerEvent,
  Processor,
  WorkerHost,
} from '@nestjs/bullmq';
import { Job, Queue } from 'bullmq';
import { Injectable } from '@nestjs/common';
import { InjectMetric } from '@willsoto/nestjs-prometheus';
import { Counter, Gauge } from 'prom-client';
import axios, { isAxiosError } from 'axios';
import * as crypto from 'crypto';
import { LoggerService } from '../common/filters/logger.service';
import { DeliveriesRepository } from './deliveries.repository';
import { EndpointsRepository } from '../endpoints/endpoints.repository';
import { assertSafeWebhookUrl } from '../security/url-safety';

@Processor('webhook-deliveries')
@Injectable()
export class DeliveryProcessor extends WorkerHost {
  constructor(
    private readonly deliveriesRepo: DeliveriesRepository,
    private readonly endpointsRepo: EndpointsRepository,
    private readonly logger: LoggerService,
    @InjectQueue('webhook-deliveries')
    private readonly queue: Queue,
    @InjectMetric('payment_reference_webhook_deliveries_total')
    private readonly deliveriesTotal: Counter<string>,
    @InjectMetric('payment_reference_webhook_dead_letter_total')
    private readonly deadLetterGauge: Gauge<string>,
    @InjectMetric('payment_reference_webhook_retry_queue_total')
    private readonly retryQueueGauge: Gauge<string>,
  ) {
    super();
  }

  async process(job: Job<{ deliveryId: string }>) {
    this.log('INFO', 'processing webhook delivery job', {
      job_id: job.id,
      delivery_id: job.data.deliveryId,
      attempt: job.attemptsMade + 1,
      max_attempts: job.opts.attempts ?? 5,
    });

    const delivery = await this.deliveriesRepo.findById(job.data.deliveryId);
    if (!delivery) {
      // Delivery record missing — log and discard
      this.log('ERROR', 'delivery record not found', {
        delivery_id: job.data.deliveryId,
      });
      this.deliveriesTotal.inc({ status: 'failed' });
      return;
    }

    const endpoint = await this.endpointsRepo.findById(delivery.endpointId);
    if (!endpoint || !endpoint.isActive) {
      // Endpoint was deleted or deactivated after job was queued — discard
      this.log('WARN', 'endpoint not found or inactive', {
        endpoint_id: delivery.endpointId,
        delivery_id: delivery.id,
      });
      await this.deliveriesRepo.markFailed(
        delivery.id,
        null,
        'endpoint_inactive',
      );
      this.deliveriesTotal.inc({ status: 'failed' });
      return;
    }

    const timestamp = Math.floor(Date.now() / 1000).toString();
    const body = JSON.stringify(delivery.payload);
    const signature = this.sign(body, endpoint.secret, timestamp);

    this.log('INFO', 'attempting webhook delivery', {
      delivery_id: delivery.id,
      endpoint_url: endpoint.url,
      attempt: job.attemptsMade + 1,
      event_type: delivery.eventType,
    });

    try {
      await assertSafeWebhookUrl(endpoint.url);
      const response = await axios.post<unknown>(
        endpoint.url,
        delivery.payload,
        {
          headers: {
            'Content-Type': 'application/json',
            'X-Webhook-Signature': `t=${timestamp},v1=${signature}`,
            'X-Webhook-ID': delivery.id,
            'X-Webhook-Event': delivery.eventType,
            'X-Delivery-Attempt': String(job.attemptsMade + 1),
          },
          timeout: 10_000, // 10 second timeout
          maxRedirects: 0,
          proxy: false,
          maxBodyLength: 1024 * 1024,
          maxContentLength: 1024 * 1024,
        },
      );

      await this.deliveriesRepo.markDelivered(
        delivery.id,
        response.status,
        this.safeStringify(response.data ?? null, 'null'),
      );

      this.log('INFO', 'webhook delivered successfully', {
        delivery_id: delivery.id,
        status_code: response.status,
        attempt: job.attemptsMade + 1,
      });
      this.deliveriesTotal.inc({ status: 'delivered' });
    } catch (error: unknown) {
      const statusCode = isAxiosError(error)
        ? (error.response?.status ?? null)
        : null;
      const responseData: unknown = isAxiosError(error)
        ? error.response?.data
        : undefined;
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      const responseBody =
        responseData !== undefined
          ? this.safeStringify(responseData, errorMessage)
          : errorMessage.slice(0, 1000);

      const isLastAttempt = job.attemptsMade >= (job.opts.attempts ?? 5) - 1;

      this.log('WARN', 'webhook delivery failed', {
        delivery_id: delivery.id,
        endpoint_url: endpoint.url,
        status_code: statusCode,
        attempt: job.attemptsMade + 1,
        is_last: isLastAttempt,
        error: errorMessage,
      });

      if (isLastAttempt) {
        // All retries exhausted — move to dead letter
        await this.deliveriesRepo.markDeadWithLetter(delivery, [
          {
            attempt: job.attemptsMade + 1,
            status_code: statusCode,
            error: responseBody,
            timestamp: new Date().toISOString(),
          },
        ]);

        this.log('ERROR', 'webhook moved to dead letter queue', {
          delivery_id: delivery.id,
          endpoint_id: endpoint.id,
          event_type: delivery.eventType,
        });
        this.deliveriesTotal.inc({ status: 'dead' });
        this.deadLetterGauge.inc();
        await this.updateRetryQueueGauge();

        return; // don't rethrow — job is done, just dead
      }

      await this.deliveriesRepo.markFailed(
        delivery.id,
        statusCode,
        responseBody,
      );
      this.deliveriesTotal.inc({ status: 'failed' });

      // Rethrow so BullMQ schedules the retry with exponential backoff
      throw error;
    }
  }

  @OnWorkerEvent('active')
  onActive() {
    void this.updateRetryQueueGauge();
  }

  @OnWorkerEvent('completed')
  onCompleted() {
    void this.updateRetryQueueGauge();
  }

  @OnWorkerEvent('failed')
  onFailed() {
    void this.updateRetryQueueGauge();
  }

  @OnWorkerEvent('drained')
  onDrained() {
    void this.updateRetryQueueGauge();
  }

  // HMAC-SHA256 signature lets merchants verify the webhook came from this stack.
  private sign(body: string, secret: string, timestamp: string): string {
    const payload = `${timestamp}.${body}`;
    return crypto.createHmac('sha256', secret).update(payload).digest('hex');
  }

  private safeStringify(value: unknown, fallback: string): string {
    try {
      return JSON.stringify(value).slice(0, 1000);
    } catch {
      return fallback.slice(0, 1000);
    }
  }

  private log(
    level: string,
    msg: string,
    fields: Record<string, unknown> = {},
  ) {
    const logger = this.logger.with({ component: 'delivery_processor' });

    switch (level) {
      case 'ERROR':
        logger.error(msg, fields);
        break;
      case 'WARN':
        logger.warn(msg, fields);
        break;
      case 'DEBUG':
        logger.debug(msg, fields);
        break;
      default:
        logger.info(msg, fields);
        break;
    }
  }

  private async updateRetryQueueGauge() {
    try {
      const counts = await this.queue.getJobCounts('delayed');
      this.retryQueueGauge.set(counts.delayed ?? 0);
    } catch (error) {
      this.log('WARN', 'failed to update webhook retry queue metric', {
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }
}
