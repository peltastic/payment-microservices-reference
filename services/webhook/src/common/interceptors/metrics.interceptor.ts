import {
  CallHandler,
  ExecutionContext,
  Injectable,
  NestInterceptor,
} from '@nestjs/common';
import { InjectMetric } from '@willsoto/nestjs-prometheus';
import { Counter, Histogram } from 'prom-client';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';
import type { Request, Response } from 'express';

@Injectable()
export class MetricsInterceptor implements NestInterceptor {
  constructor(
    @InjectMetric('http_requests_total')
    private readonly requestsTotal: Counter<string>,
    @InjectMetric('http_request_duration_seconds')
    private readonly requestDuration: Histogram<string>,
  ) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const httpContext = context.switchToHttp();
    const request = httpContext.getRequest<Request>();
    const response = httpContext.getResponse<Response>();
    const path = request.route?.path || request.path || 'unknown';
    const timer = this.requestDuration.startTimer({
      method: request.method,
      path,
    });

    return next.handle().pipe(
      finalize(() => {
        this.requestsTotal.inc({
          method: request.method,
          path,
          status: response.statusCode.toString(),
        });
        timer();
      }),
    );
  }
}
