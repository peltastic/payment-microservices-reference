import {
  CallHandler,
  ExecutionContext,
  Injectable,
  NestInterceptor,
} from '@nestjs/common';
import type { Request, Response } from 'express';
import { randomUUID } from 'node:crypto';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';
import { LoggerService } from './logger.service';

type RequestWithContext = Request & {
  requestId?: string;
  merchantId?: string;
};

@Injectable()
export class RequestIdInterceptor implements NestInterceptor {
  constructor(private readonly logger: LoggerService) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const httpContext = context.switchToHttp();
    const request = httpContext.getRequest<RequestWithContext>();
    const response = httpContext.getResponse<Response>();

    const requestId = request.header('x-request-id') ?? `req_${randomUUID()}`;
    const merchantId = request.header('x-merchant-id') ?? '';
    const path = request.originalUrl || request.url;
    const startedAt = Date.now();

    request.requestId = requestId;
    request.merchantId = merchantId;
    response.setHeader('x-request-id', requestId);

    const log = this.logger.with({
      component: 'http_request',
      request_id: requestId,
      merchant_id: merchantId,
      method: request.method,
      path,
      client_ip: request.ip,
    });

    log.info('request started');

    return next.handle().pipe(
      finalize(() => {
        log.info('request completed', {
          status: response.statusCode,
          duration_ms: Date.now() - startedAt,
        });
      }),
    );
  }
}
