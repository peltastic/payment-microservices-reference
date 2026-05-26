import { Injectable } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import type { WebhookEndpoint } from '../generated/prisma/client';
import { LoggerService } from '../common/filters/logger.service';
import {
  decryptSecret,
  encryptSecret,
  isEncryptedSecret,
} from '../security/secret-crypto';

@Injectable()
export class EndpointsRepository {
  constructor(
    private readonly prisma: PrismaService,
    private readonly logger: LoggerService,
  ) {}

  async create(data: {
    id: string;
    merchantId: string;
    url: string;
    secret: string;
    events: string[];
    description?: string;
  }): Promise<WebhookEndpoint> {
    const endpoint = await this.prisma.webhookEndpoint.create({
      data: { ...data, secret: encryptSecret(data.secret) },
    });
    this.logger
      .with({
        component: 'endpoints_repository',
        merchant_id: endpoint.merchantId,
        endpoint_id: endpoint.id,
      })
      .debug('webhook endpoint row inserted', {
        event_count: endpoint.events.length,
        is_active: endpoint.isActive,
      });
    return { ...endpoint, secret: data.secret };
  }

  async findAllByMerchant(merchantId: string): Promise<WebhookEndpoint[]> {
    const endpoints = await this.prisma.webhookEndpoint.findMany({
      where: { merchantId },
      orderBy: { createdAt: 'desc' },
    });
    this.logger
      .with({
        component: 'endpoints_repository',
        merchant_id: merchantId,
      })
      .debug('webhook endpoint rows loaded', { count: endpoints.length });
    return Promise.all(
      endpoints.map((endpoint) => this.decryptEndpoint(endpoint)),
    );
  }

  async findById(id: string): Promise<WebhookEndpoint | null> {
    const endpoint = await this.prisma.webhookEndpoint.findUnique({
      where: { id },
    });
    this.logger
      .with({
        component: 'endpoints_repository',
        endpoint_id: id,
        merchant_id: endpoint?.merchantId,
      })
      .debug('webhook endpoint row loaded', { found: endpoint !== null });
    return endpoint ? this.decryptEndpoint(endpoint) : null;
  }

  // Called by the Kafka consumer — finds endpoints subscribed to a specific event
  async findActiveForEvent(
    merchantId: string,
    eventType: string,
  ): Promise<WebhookEndpoint[]> {
    const endpoints = await this.prisma.webhookEndpoint.findMany({
      where: {
        merchantId,
        isActive: true,
        events: { has: eventType }, // Prisma array contains check
      },
    });
    this.logger
      .with({
        component: 'endpoints_repository',
        merchant_id: merchantId,
        event_type: eventType,
      })
      .debug('active webhook endpoint rows loaded', {
        count: endpoints.length,
      });
    return Promise.all(
      endpoints.map((endpoint) => this.decryptEndpoint(endpoint)),
    );
  }

  async update(
    id: string,
    merchantId: string,
    data: {
      url?: string;
      events?: string[];
      isActive?: boolean;
      description?: string | null;
    },
  ): Promise<WebhookEndpoint | null> {
    const endpoint = await this.prisma.webhookEndpoint.findFirst({
      where: { id, merchantId },
      select: { id: true },
    });

    if (!endpoint) return null;

    const updated = await this.prisma.webhookEndpoint.update({
      where: { id },
      data,
    });
    this.logger
      .with({
        component: 'endpoints_repository',
        merchant_id: updated.merchantId,
        endpoint_id: id,
      })
      .debug('webhook endpoint row updated', {
        is_active: updated.isActive,
        event_count: updated.events.length,
      });
    return this.decryptEndpoint(updated);
  }

  async delete(id: string, merchantId: string): Promise<boolean> {
    const { count } = await this.prisma.webhookEndpoint.deleteMany({
      where: { id, merchantId },
    });

    this.logger
      .with({
        component: 'endpoints_repository',
        merchant_id: merchantId,
        endpoint_id: id,
      })
      .debug('webhook endpoint rows deleted', { count });

    return count > 0;
  }

  private async decryptEndpoint(
    endpoint: WebhookEndpoint,
  ): Promise<WebhookEndpoint> {
    if (!isEncryptedSecret(endpoint.secret)) {
      await this.prisma.webhookEndpoint.update({
        where: { id: endpoint.id },
        data: { secret: encryptSecret(endpoint.secret) },
      });
      return endpoint;
    }

    return { ...endpoint, secret: decryptSecret(endpoint.secret) };
  }
}
