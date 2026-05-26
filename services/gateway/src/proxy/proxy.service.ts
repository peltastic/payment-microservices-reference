import { HttpService } from '@nestjs/axios';
import { Injectable, ServiceUnavailableException } from '@nestjs/common';
import type { Request } from 'express';
import { firstValueFrom } from 'rxjs';
import { LoggerService } from '../common/filters/logger.service';
import { InjectMetric } from '@willsoto/nestjs-prometheus';
import { Counter } from 'prom-client';
import { signInternalRequest } from '../security/internal-signature';

type GatewayRequest = Request & {
  requestId?: string;
  merchantId?: string;
  merchantScope?: string;
};

@Injectable()
export class ProxyService {
  constructor(
    private readonly httpService: HttpService,
    private readonly logger: LoggerService,
    @InjectMetric('payment_reference_proxy_requests_total')
    private readonly proxyRequestsTotal: Counter<string>,
  ) {}

  async forward(
    request: GatewayRequest,
    targetService: string,
    targetUrl: string,
  ) {
    const log = this.logger.with({
      component: 'proxy_service',
      request_id: request.requestId,
      merchant_id: request.merchantId,
      method: request.method,
      path: request.originalUrl || request.url,
      target_service: targetService,
      target_url: targetUrl,
    });

    try {
      const signedTargetUrl = this.withQuery(targetUrl, request.query);
      const headers = {
        'content-type': 'application/json',
        'x-merchant-id': request.merchantId,
        'x-merchant-scope': request.merchantScope,
        'x-request-id': request.requestId,
        ...signInternalRequest({
          method: request.method,
          targetUrl: signedTargetUrl,
          requestId: request.requestId,
          merchantId: request.merchantId,
          body: request.body,
        }),
      };

      log.info('forwarding request downstream', {
        query: request.query,
        target_url: signedTargetUrl,
        has_body: request.body !== undefined && request.body !== null,
      });

      const response = await firstValueFrom(
        this.httpService.request({
          method: request.method,
          url: signedTargetUrl,
          headers,
          data: request.body,
          validateStatus: () => true,
        }),
      );

      log.info('downstream response received', {
        status: response.status,
      });
      this.proxyRequestsTotal.inc({
        service: targetService,
        status: response.status.toString(),
      });

      return { status: response.status, data: response.data };
    } catch (error) {
      this.proxyRequestsTotal.inc({
        service: targetService,
        status: 'failed',
      });
      log.error('downstream request failed', {
        error: error instanceof Error ? error.message : String(error),
      });
      throw new ServiceUnavailableException({
        error: {
          code: 'downstream_unavailable',
          message: `${targetService} service is temporarily unavailable`,
        },
      });
    }
  }

  private withQuery(targetUrl: string, query: Request['query']): string {
    const url = new URL(targetUrl);

    for (const [key, value] of Object.entries(query)) {
      if (Array.isArray(value)) {
        for (const entry of value) {
          if (entry !== undefined) {
            url.searchParams.append(key, String(entry));
          }
        }
        continue;
      }

      if (value !== undefined) {
        url.searchParams.append(key, String(value));
      }
    }

    return url.toString();
  }
}
