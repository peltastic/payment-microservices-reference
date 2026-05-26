import { Injectable, NotFoundException } from '@nestjs/common';
import { EndpointsRepository } from './endpoints.repository';
import { CreateEndpointDto } from './dto/create-endpoint.dto';
import { UpdateEndpointDto } from './dto/update-endpoint.dto';
import type { WebhookEndpoint } from '../generated/prisma/client';
import { LoggerService } from '../common/filters/logger.service';
import { ulid } from 'ulid';
import * as crypto from 'crypto';
import { assertSafeWebhookUrl } from '../security/url-safety';

type EndpointWithoutSecret = Omit<WebhookEndpoint, 'secret'>;

@Injectable()
export class EndpointsService {
  constructor(
    private readonly repo: EndpointsRepository,
    private readonly logger: LoggerService,
  ) {}

  async create(
    merchantId: string,
    dto: CreateEndpointDto,
  ): Promise<WebhookEndpoint> {
    const log = this.logger.with({
      component: 'endpoints_service',
      merchant_id: merchantId,
    });
    await assertSafeWebhookUrl(dto.url);

    const secret = `whsec_${crypto.randomBytes(32).toString('hex')}`;

    const endpoint = await this.repo.create({
      id: ulid(),
      merchantId,
      url: dto.url,
      secret,
      events: dto.events,
      description: dto.description,
    });

    log.info('webhook endpoint created', {
      endpoint_id: endpoint.id,
      url: endpoint.url,
      event_count: endpoint.events.length,
      is_active: endpoint.isActive,
    });

    // Secret is returned here and ONLY here — never again after this
    return endpoint;
  }

  async findAll(merchantId: string): Promise<EndpointWithoutSecret[]> {
    const log = this.logger.with({
      component: 'endpoints_service',
      merchant_id: merchantId,
    });
    const endpoints = await this.repo.findAllByMerchant(merchantId);
    log.info('webhook endpoints listed', { count: endpoints.length });

    // Strip secret from every item in list response
    return endpoints.map((endpoint) => this.stripSecret(endpoint));
  }

  async update(
    id: string,
    merchantId: string,
    dto: UpdateEndpointDto,
  ): Promise<EndpointWithoutSecret> {
    const log = this.logger.with({
      component: 'endpoints_service',
      merchant_id: merchantId,
      endpoint_id: id,
    });
    if (dto.url !== undefined) {
      await assertSafeWebhookUrl(dto.url);
    }

    const endpoint = await this.repo.update(id, merchantId, {
      url: dto.url,
      events: dto.events,
      description: dto.description,
      isActive: dto.isActive,
    });

    if (!endpoint) {
      log.warn('webhook endpoint update missed target');
      throw new NotFoundException({
        error: 'endpoint_not_found',
        message: 'Webhook endpoint not found',
      });
    }

    log.info('webhook endpoint updated', {
      url: endpoint.url,
      event_count: endpoint.events.length,
      is_active: endpoint.isActive,
    });

    return this.stripSecret(endpoint);
  }

  async remove(id: string, merchantId: string): Promise<void> {
    const log = this.logger.with({
      component: 'endpoints_service',
      merchant_id: merchantId,
      endpoint_id: id,
    });
    const deleted = await this.repo.delete(id, merchantId);

    if (!deleted) {
      log.warn('webhook endpoint delete missed target');
      throw new NotFoundException({
        error: 'endpoint_not_found',
        message: 'Webhook endpoint not found',
      });
    }

    log.info('webhook endpoint removed');
  }

  async findActiveForEvent(
    merchantId: string,
    eventType: string,
  ): Promise<WebhookEndpoint[]> {
    const endpoints = await this.repo.findActiveForEvent(merchantId, eventType);
    this.logger
      .with({
        component: 'endpoints_service',
        merchant_id: merchantId,
        event_type: eventType,
      })
      .info('active webhook endpoints selected for event', {
        endpoint_count: endpoints.length,
      });
    return endpoints;
  }

  private stripSecret(endpoint: WebhookEndpoint): EndpointWithoutSecret {
    const {
      id,
      merchantId,
      url,
      events,
      isActive,
      description,
      createdAt,
      updatedAt,
    } = endpoint;

    return {
      id,
      merchantId,
      url,
      events,
      isActive,
      description,
      createdAt,
      updatedAt,
    };
  }
}
