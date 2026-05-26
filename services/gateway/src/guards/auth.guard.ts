import { HttpService } from '@nestjs/axios';
import {
  CanActivate,
  ExecutionContext,
  ForbiddenException,
  Injectable,
  ServiceUnavailableException,
  UnauthorizedException,
} from '@nestjs/common';
import type { Request, Response } from 'express';
import { ulid } from 'ulid';
import { firstValueFrom } from 'rxjs';
import { isAxiosError } from 'axios';
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
export class AuthGuard implements CanActivate {
  constructor(
    private readonly httpService: HttpService,
    private readonly logger: LoggerService,
    @InjectMetric('payment_reference_auth_validations_total')
    private readonly validationsTotal: Counter<string>,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const httpContext = context.switchToHttp();
    const request = httpContext.getRequest<GatewayRequest>();
    const response = httpContext.getResponse<Response>();
    const requestId =
      request.requestId ?? request.header('x-request-id') ?? `req_${ulid()}`;
    const authHeader = request.headers['authorization'];

    request.requestId = requestId;
    response.setHeader('x-request-id', requestId);

    const log = this.logger.with({
      component: 'auth_guard',
      request_id: requestId,
      method: request.method,
      path: request.originalUrl || request.url,
    });

    if (this.isUnauthenticatedPath(request)) {
      log.debug(
        'skipping gateway authentication for internal health/metrics path',
      );
      return true;
    }

    if (!authHeader?.startsWith('Bearer ')) {
      this.validationsTotal.inc({ result: 'invalid' });
      log.warn('gateway authentication failed: missing bearer token');
      throw new UnauthorizedException({
        error: {
          code: 'missing_key',
          message: 'Authorization header required',
        },
      });
    }

    const apiKey = authHeader.split(' ')[1];

    log.info('validating API key with auth service', {
      auth_service_url: process.env.AUTH_SERVICE_URL,
    });

    try {
      const targetUrl = `${process.env.AUTH_SERVICE_URL}/internal/validate`;
      const validationBody = { api_key: apiKey };
      const { data } = await firstValueFrom(
        this.httpService.post(targetUrl, validationBody, {
          headers: {
            'x-request-id': request.requestId,
            ...signInternalRequest({
              method: 'POST',
              targetUrl,
              requestId: request.requestId,
              body: validationBody,
            }),
          },
        }),
      );
      request.merchantId = data.merchant_id ?? data.MerchantID;
      request.merchantScope = data.scope ?? data.Scope;
      this.validationsTotal.inc({ result: 'valid' });

      const requiredScopes = this.requiredScopes(request);
      if (
        requiredScopes.length > 0 &&
        !this.hasAnyScope(request.merchantScope, requiredScopes)
      ) {
        log.warn('gateway authorization failed: insufficient scope', {
          merchant_id: request.merchantId,
          merchant_scope: request.merchantScope,
          required_scopes: requiredScopes,
        });
        throw new ForbiddenException({
          error: {
            code: 'insufficient_scope',
            message: 'API key scope does not allow this operation',
          },
        });
      }

      log.info('gateway authentication succeeded', {
        merchant_id: request.merchantId,
        merchant_scope: request.merchantScope,
      });

      return true;
    } catch (error) {
      if (error instanceof ForbiddenException) {
        throw error;
      }

      const result = this.authValidationResult(error);
      this.validationsTotal.inc({ result });

      if (result === 'unavailable') {
        log.error('gateway authentication failed: auth service unavailable', {
          error: error instanceof Error ? error.message : String(error),
        });
        throw new ServiceUnavailableException({
          error: {
            code: 'auth_service_unavailable',
            message: 'Authentication service is temporarily unavailable',
          },
        });
      }

      log.warn('gateway authentication failed: invalid or revoked API key', {
        error: error instanceof Error ? error.message : String(error),
      });
      throw new UnauthorizedException({
        error: {
          code: 'invalid_key',
          message: 'API key is invalid or revoked',
        },
      });
    }
  }

  private isUnauthenticatedPath(request: GatewayRequest): boolean {
    const path = (request.originalUrl || request.url).split('?')[0];
    if (path === '/health') {
      return true;
    }

    return path === '/metrics' && this.isInternalClient(request);
  }

  private isInternalClient(request: GatewayRequest): boolean {
    const address = (request.ip || request.socket.remoteAddress || '').replace(
      /^::ffff:/,
      '',
    );

    return (
      address === '::1' ||
      address === '127.0.0.1' ||
      address.startsWith('10.') ||
      address.startsWith('192.168.') ||
      /^172\.(1[6-9]|2\d|3[0-1])\./.test(address)
    );
  }

  private requiredScopes(request: GatewayRequest): string[] {
    const path = (request.originalUrl || request.url).split('?')[0];
    const method = request.method.toUpperCase();

    if (path.startsWith('/v1/auth')) {
      return ['auth:write'];
    }

    if (path.startsWith('/v1/payments')) {
      return method === 'GET'
        ? ['payments:read', 'payments:write']
        : ['payments:write'];
    }

    if (path.startsWith('/v1/balance')) {
      return ['ledger:read', 'ledger:write'];
    }

    if (path.startsWith('/v1/transactions')) {
      return ['ledger:write'];
    }

    if (path.startsWith('/v1/webhooks')) {
      return method === 'GET'
        ? ['webhooks:read', 'webhooks:write']
        : ['webhooks:write'];
    }

    return [];
  }

  private hasAnyScope(
    scope: string | undefined,
    requiredScopes: string[],
  ): boolean {
    const scopes = (scope ?? '')
      .split(/[\s,]+/)
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

    return (
      scopes.includes('full') ||
      scopes.includes('admin') ||
      requiredScopes.some((required) => scopes.includes(required))
    );
  }

  private authValidationResult(
    error: unknown,
  ): 'invalid' | 'revoked' | 'unavailable' {
    if (isAxiosError(error)) {
      const response = error.response;
      if (!response || response.status >= 500) {
        return 'unavailable';
      }
      const data = response?.data;
      const code =
        typeof data === 'object' &&
        data !== null &&
        'error' in data &&
        typeof (data as { error?: { code?: unknown } }).error?.code === 'string'
          ? (data as { error: { code: string } }).error.code
          : undefined;

      if (response?.status === 403 || code === 'key_revoked') {
        return 'revoked';
      }
    }

    return 'invalid';
  }
}
