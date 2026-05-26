import { Injectable } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import type { WebhookDelivery } from '../generated/prisma/client';
import { LoggerService } from '../common/filters/logger.service';
import { createHash } from 'node:crypto';
import { ulid } from 'ulid';

@Injectable()
export class DeliveriesRepository {
  constructor(
    private readonly prisma: PrismaService,
    private readonly logger: LoggerService,
  ) {}

  async create(data: {
    endpointId: string;
    merchantId: string;
    eventId: string;
    eventType: string;
    payload: object;
  }): Promise<WebhookDelivery> {
    const id = this.deliveryId(data.endpointId, data.eventId);
    const delivery = await this.prisma.webhookDelivery.upsert({
      where: { id },
      create: {
        id,
        status: 'pending',
        ...data,
      },
      update: {},
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        merchant_id: delivery.merchantId,
        endpoint_id: delivery.endpointId,
        delivery_id: delivery.id,
        event_id: delivery.eventId,
        event_type: delivery.eventType,
      })
      .debug('webhook delivery row upserted', { status: delivery.status });
    return delivery;
  }

  async findById(id: string): Promise<WebhookDelivery | null> {
    const delivery = await this.prisma.webhookDelivery.findUnique({
      where: { id },
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        delivery_id: id,
        merchant_id: delivery?.merchantId,
        endpoint_id: delivery?.endpointId,
      })
      .debug('webhook delivery row loaded', { found: delivery !== null });
    return delivery;
  }

  async findByIdForMerchant(
    id: string,
    merchantId: string,
  ): Promise<WebhookDelivery | null> {
    const delivery = await this.prisma.webhookDelivery.findFirst({
      where: { id, merchantId },
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        delivery_id: id,
        merchant_id: merchantId,
        endpoint_id: delivery?.endpointId,
      })
      .debug('merchant webhook delivery row loaded', {
        found: delivery !== null,
      });
    return delivery;
  }

  async findAllByMerchant(merchantId: string, page: number, limit: number) {
    const [data, total] = await this.prisma.$transaction([
      this.prisma.webhookDelivery.findMany({
        where: { merchantId },
        orderBy: { createdAt: 'desc' },
        skip: (page - 1) * limit,
        take: limit,
        include: { endpoint: { select: { url: true } } },
      }),
      this.prisma.webhookDelivery.count({ where: { merchantId } }),
    ]);

    this.logger
      .with({
        component: 'deliveries_repository',
        merchant_id: merchantId,
      })
      .debug('webhook delivery rows loaded', {
        page,
        limit,
        count: data.length,
        total,
      });

    return { data, total, page, limit };
  }

  async markDelivered(
    id: string,
    statusCode: number,
    response: string,
  ): Promise<void> {
    await this.prisma.webhookDelivery.update({
      where: { id },
      data: {
        status: 'delivered',
        lastStatusCode: statusCode,
        lastResponse: response,
        attemptCount: { increment: 1 },
      },
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        delivery_id: id,
      })
      .debug('webhook delivery marked delivered', { status_code: statusCode });
  }

  async markFailed(
    id: string,
    statusCode: number | null,
    response: string,
  ): Promise<void> {
    await this.prisma.webhookDelivery.update({
      where: { id },
      data: {
        status: 'failed',
        lastStatusCode: statusCode,
        lastResponse: response,
        attemptCount: { increment: 1 },
      },
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        delivery_id: id,
      })
      .debug('webhook delivery marked failed', { status_code: statusCode });
  }

  async markDeadWithLetter(
    delivery: WebhookDelivery,
    failureLog: object[],
  ): Promise<void> {
    const deadLetterId = ulid();
    await this.prisma.$transaction([
      this.prisma.webhookDelivery.update({
        where: { id: delivery.id },
        data: { status: 'dead' },
      }),
      this.prisma.deadLetterEvent.upsert({
        where: { deliveryId: delivery.id },
        create: {
          id: deadLetterId,
          deliveryId: delivery.id,
          endpointId: delivery.endpointId,
          merchantId: delivery.merchantId,
          eventType: delivery.eventType,
          payload: delivery.payload as object,
          failureLog,
        },
        update: {
          failureLog,
        },
      }),
    ]);
    this.logger
      .with({
        component: 'deliveries_repository',
        merchant_id: delivery.merchantId,
        endpoint_id: delivery.endpointId,
        delivery_id: delivery.id,
        event_type: delivery.eventType,
      })
      .debug('webhook delivery marked dead with dead letter event', {
        dead_letter_id: deadLetterId,
        failure_count: failureLog.length,
      });
  }

  async findDeadLetters(merchantId: string) {
    const deadLetters = await this.prisma.deadLetterEvent.findMany({
      where: { merchantId },
      orderBy: { createdAt: 'desc' },
    });
    this.logger
      .with({
        component: 'deliveries_repository',
        merchant_id: merchantId,
      })
      .debug('dead letter event rows loaded', { count: deadLetters.length });
    return deadLetters;
  }

  private deliveryId(endpointId: string, eventId: string): string {
    const hash = createHash('sha256')
      .update(`${endpointId}:${eventId}`)
      .digest('hex')
      .slice(0, 26);
    return `whd_${hash}`;
  }
}
