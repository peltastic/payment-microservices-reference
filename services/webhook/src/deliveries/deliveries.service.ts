import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectQueue } from '@nestjs/bullmq';
import { Queue } from 'bullmq';
import { LoggerService } from '../common/filters/logger.service';
import { DeliveriesRepository } from './deliveries.repository';

@Injectable()
export class DeliveriesService {
  constructor(
    private readonly repo: DeliveriesRepository,
    @InjectQueue('webhook-deliveries') private readonly queue: Queue,
    private readonly logger: LoggerService,
  ) {}

  async enqueue(data: {
    endpointId: string;
    merchantId: string;
    eventId: string;
    eventType: string;
    payload: object;
  }) {
    const log = this.logger.with({
      component: 'deliveries_service',
      merchant_id: data.merchantId,
      endpoint_id: data.endpointId,
      event_id: data.eventId,
      event_type: data.eventType,
    });

    // Create delivery record first
    const delivery = await this.repo.create(data);
    if (delivery.status === 'delivered' || delivery.status === 'dead') {
      log.info('webhook delivery already terminal, skipping enqueue', {
        delivery_id: delivery.id,
        status: delivery.status,
        queue: 'webhook-deliveries',
      });
      return delivery;
    }

    // Push to BullMQ
    await this.queue.add(
      'deliver',
      { deliveryId: delivery.id },
      {
        attempts: 5,
        backoff: {
          type: 'exponential',
          delay: 30_000, // 30s → 1m → 2m → 4m → 8m
        },
        jobId: delivery.id,
        removeOnComplete: false, // keep for audit
        removeOnFail: false,
      },
    );

    log.info('webhook delivery enqueued', {
      delivery_id: delivery.id,
      queue: 'webhook-deliveries',
    });

    return delivery;
  }

  async findAll(merchantId: string, page = 1, limit = 20) {
    const deliveries = await this.repo.findAllByMerchant(
      merchantId,
      page,
      limit,
    );

    this.logger
      .with({
        component: 'deliveries_service',
        merchant_id: merchantId,
      })
      .info('webhook deliveries listed', {
        page,
        limit,
        count: deliveries.data.length,
        total: deliveries.total,
      });

    return deliveries;
  }

  async findDeadLetters(merchantId: string) {
    const deadLetters = await this.repo.findDeadLetters(merchantId);

    this.logger
      .with({
        component: 'deliveries_service',
        merchant_id: merchantId,
      })
      .info('dead letter deliveries listed', {
        count: deadLetters.length,
      });

    return deadLetters;
  }

  async retryDelivery(deliveryId: string, merchantId: string) {
    const delivery = await this.repo.findByIdForMerchant(
      deliveryId,
      merchantId,
    );

    if (!delivery) {
      throw new NotFoundException({
        error: {
          code: 'delivery_not_found',
          message: 'Webhook delivery not found',
        },
      });
    }

    await this.queue.add(
      'deliver',
      { deliveryId },
      {
        attempts: 5,
        backoff: { type: 'exponential', delay: 30_000 },
        jobId: `manual-retry:${deliveryId}:${Date.now()}`,
      },
    );

    this.logger
      .with({
        component: 'deliveries_service',
        delivery_id: deliveryId,
        merchant_id: merchantId,
      })
      .info('webhook delivery retry queued', {
        queue: 'webhook-deliveries',
      });
  }
}
