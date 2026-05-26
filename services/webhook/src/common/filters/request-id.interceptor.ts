import {
  Injectable,
  NestInterceptor,
  ExecutionContext,
  CallHandler,
} from '@nestjs/common';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';
import { ulid } from 'ulid';
import type { Request, Response } from 'express';
import { LoggerService } from './logger.service';

type RequestWithId = Request & {
  requestId?: string;
  merchantId?: string;
};

@Injectable()
export class RequestIdInterceptor implements NestInterceptor {
  constructor(private readonly logger: LoggerService) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const httpContext = context.switchToHttp();
    const request = httpContext.getRequest<RequestWithId>();
    const response = httpContext.getResponse<Response>();

    const requestId = request.header('x-request-id') ?? `req_${ulid()}`;
    const merchantId = request.header('x-merchant-id');
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
