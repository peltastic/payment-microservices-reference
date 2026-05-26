import {
  Injectable,
  NestInterceptor,
  ExecutionContext,
  CallHandler,
} from '@nestjs/common';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';
import { InjectMetric } from '@willsoto/nestjs-prometheus';
import { Counter, Histogram } from 'prom-client';

@Injectable()
export class MetricsInterceptor implements NestInterceptor {
  constructor(
    @InjectMetric('http_requests_total')
    private readonly requestsTotal: Counter<string>,

    @InjectMetric('http_request_duration_seconds')
    private readonly requestDuration: Histogram<string>,
  ) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<any> {
    const request = context.switchToHttp().getRequest();
    const response = context.switchToHttp().getResponse();
    const start = Date.now();

    // Get route pattern not actual path — prevents cardinality explosion
    // /payments/:id not /payments/pay_01HXYZ01HXYZ01HXYZ
    const path = request.route?.path || request.path;

    const timer = this.requestDuration.startTimer({
      method: request.method,
      path,
    });

    return next.handle().pipe(
      finalize(() => {
        const status = response.statusCode.toString();

        this.requestsTotal.inc({
          method: request.method,
          path,
          status,
        });

        timer(); // stops the histogram timer and records the value
      }),
    );
  }
}
