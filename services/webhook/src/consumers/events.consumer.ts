import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { Kafka, Consumer, EachMessagePayload } from 'kafkajs';
import { createHmac, timingSafeEqual } from 'node:crypto';
import { LoggerService } from '../common/filters/logger.service';
import { EndpointsService } from '../endpoints/endpoints.service';
import { DeliveriesService } from '../deliveries/deliveries.service';

interface PaymentEvent {
  id: string;
  type: string;
  version: string;
  timestamp: string;
  source: string;
  data: {
    payment_id: string;
    merchant_id: string;
    amount: number;
    currency: string;
    customer_email: string;
    customer_name: string;
    bank_reference: string;
    failure_reason: string;
  };
}

@Injectable()
export class EventsConsumer implements OnModuleInit, OnModuleDestroy {
  private consumer: Consumer;

  private readonly TOPICS = parseTopics(
    process.env.KAFKA_TOPICS || process.env.KAFKA_TOPIC_PAYMENTS || 'payments',
  );

  constructor(
    private readonly endpointsService: EndpointsService,
    private readonly deliveriesService: DeliveriesService,
    private readonly logger: LoggerService,
  ) {
    const kafka = new Kafka({
      clientId: 'webhook-service',
      brokers: (process.env.KAFKA_BROKERS || 'kafka:9092').split(','),
      retry: {
        initialRetryTime: 1000,
        retries: 10,
      },
    });

    this.consumer = kafka.consumer({
      groupId: process.env.KAFKA_GROUP_ID || 'webhook-service',
    });
  }

  async onModuleInit() {
    this.log('INFO', 'connecting kafka consumer', {
      topics: this.TOPICS,
    });
    await this.consumer.connect();

    // Subscribe to all topics
    for (const topic of this.TOPICS) {
      await this.consumer.subscribe({ topic, fromBeginning: false });
    }

    await this.consumer.run({
      // Process one message at a time per partition
      // Prevents overwhelming merchant endpoints with concurrent deliveries
      partitionsConsumedConcurrently: 3,

      eachMessage: async (payload: EachMessagePayload) => {
        await this.handleMessage(payload);
      },
    });

    this.log('INFO', 'kafka consumer started', { topics: this.TOPICS });
  }

  async onModuleDestroy() {
    await this.consumer.disconnect();
    this.log('INFO', 'kafka consumer disconnected');
  }

  private async handleMessage({
    topic,
    partition,
    message,
  }: EachMessagePayload) {
    if (!message.value) {
      this.log('WARN', 'received empty message', { topic, partition });
      return;
    }

    let event: PaymentEvent;
    const payload = message.value;

    if (!this.validEventSignature(payload, message.headers)) {
      this.log('WARN', 'kafka message rejected due to invalid signature', {
        topic,
        partition,
        offset: message.offset,
      });
      return;
    }

    try {
      event = JSON.parse(payload.toString()) as PaymentEvent;
    } catch (err) {
      // Malformed JSON — log and skip, never block the consumer
      this.log('ERROR', 'failed to parse kafka message', {
        topic,
        offset: message.offset,
        error: String(err),
      });
      return;
    }

    this.log('INFO', 'kafka message received', {
      topic,
      event_id: event.id,
      event_type: event.type,
      merchant_id: event.data?.merchant_id,
      offset: message.offset,
    });

    try {
      await this.dispatch(event);
    } catch (err) {
      this.log('ERROR', 'failed to dispatch event to endpoints', {
        event_id: event.id,
        topic,
        error: String(err),
      });
      throw err;
    }
  }

  private async dispatch(event: PaymentEvent) {
    const merchantId = event.data?.merchant_id;
    if (!merchantId) {
      this.log('WARN', 'event missing merchant_id, skipping', {
        event_id: event.id,
      });
      return;
    }

    // Find all active endpoints subscribed to this event type
    const endpoints = await this.endpointsService.findActiveForEvent(
      merchantId,
      event.type,
    );

    if (endpoints.length === 0) {
      this.log('INFO', 'no active endpoints for event', {
        event_id: event.id,
        event_type: event.type,
        merchant_id: merchantId,
      });
      return;
    }

    this.log('INFO', 'dispatching event to endpoints', {
      event_id: event.id,
      event_type: event.type,
      merchant_id: merchantId,
      endpoint_count: endpoints.length,
    });

    // Create an idempotent delivery job for each subscribed endpoint.
    // If any enqueue fails, rethrow so Kafka does not advance the offset.
    const dispatches = endpoints.map((endpoint) =>
      this.deliveriesService.enqueue({
        endpointId: endpoint.id,
        merchantId,
        eventId: event.id,
        eventType: event.type,
        payload: event,
      }),
    );

    const results = await Promise.allSettled(dispatches);
    const failures = results
      .map((result, index) => ({ result, endpoint: endpoints[index] }))
      .filter(
        (
          item,
        ): item is {
          result: PromiseRejectedResult;
          endpoint: (typeof endpoints)[number];
        } => item.result.status === 'rejected',
      );
    if (failures.length > 0) {
      failures.forEach((failure) => {
        this.log('ERROR', 'failed to enqueue delivery', {
          event_id: event.id,
          endpoint_id: failure.endpoint.id,
          error: String(failure.result.reason),
        });
      });
      throw new Error(
        `failed to enqueue ${failures.length} of ${endpoints.length} webhook deliveries`,
      );
    }

    this.log('INFO', 'event dispatch scheduling completed', {
      event_id: event.id,
      event_type: event.type,
      merchant_id: merchantId,
      endpoint_count: endpoints.length,
    });
  }

  private validEventSignature(
    payload: Buffer,
    headers: EachMessagePayload['message']['headers'],
  ): boolean {
    const secret =
      process.env.EVENT_SIGNING_SECRET || process.env.INTERNAL_AUTH_SECRET;
    const signature = this.headerValue(headers, 'x-event-signature');

    if (!secret || !signature) {
      return false;
    }

    const expected = createHmac('sha256', secret).update(payload).digest('hex');
    const provided = Buffer.from(signature);
    const calculated = Buffer.from(expected);

    return (
      provided.length === calculated.length &&
      timingSafeEqual(provided, calculated)
    );
  }

  private headerValue(
    headers: EachMessagePayload['message']['headers'],
    key: string,
  ): string {
    const value = headers?.[key];
    const entry = Array.isArray(value) ? value[value.length - 1] : value;

    if (!entry) {
      return '';
    }

    return Buffer.isBuffer(entry) ? entry.toString() : String(entry);
  }

  private log(
    level: string,
    msg: string,
    fields: Record<string, unknown> = {},
  ) {
    const logger = this.logger.with({ component: 'events_consumer' });

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
}

function parseTopics(value: string): string[] {
  const topics = value
    .split(',')
    .map((topic) => topic.trim())
    .filter((topic) => topic.length > 0);

  return Array.from(new Set(topics.length > 0 ? topics : ['payments']));
}
