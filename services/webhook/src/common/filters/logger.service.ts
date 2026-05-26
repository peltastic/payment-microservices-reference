import { Injectable, Scope } from '@nestjs/common';
import { trace } from '@opentelemetry/api';
import winston from 'winston';

@Injectable({ scope: Scope.DEFAULT })
export class LoggerService {
  private readonly serviceName = process.env.OTEL_SERVICE_NAME || 'webhook';
  private readonly logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    format: winston.format.json(),
    transports: [new winston.transports.Console()],
  });

  private getTraceContext(): Record<string, string> {
    const span = trace.getActiveSpan();
    const spanCtx = span?.spanContext();

    if (!spanCtx) {
      return {
        trace_id: '',
        span_id: '',
      };
    }

    return {
      trace_id: spanCtx.traceId,
      span_id: spanCtx.spanId,
    };
  }

  info(msg: string, fields: Record<string, any> = {}) {
    this.write('info', msg, fields);
  }

  error(msg: string, fields: Record<string, any> = {}) {
    this.write('error', msg, fields);
  }

  warn(msg: string, fields: Record<string, any> = {}) {
    this.write('warn', msg, fields);
  }

  debug(msg: string, fields: Record<string, any> = {}) {
    if (process.env.LOG_LEVEL !== 'debug') return;
    this.write('debug', msg, fields);
  }

  with(fields: Record<string, any>): BoundLogger {
    return new BoundLogger(this, fields);
  }

  private write(level: string, msg: string, fields: Record<string, any>) {
    this.logger.log({
      time: new Date().toISOString(),
      env: process.env.ENVIRONMENT || 'development',
      request_id: '',
      merchant_id: '',
      duration_ms: 0,
      component: '',
      trace_id: '',
      span_id: '',
      ...this.getTraceContext(),
      ...fields,
      level,
      service: this.serviceName,
      message: msg,
    });
  }
}

export class BoundLogger {
  constructor(
    private readonly logger: LoggerService,
    private readonly boundFields: Record<string, any>,
  ) {}

  info(msg: string, fields: Record<string, any> = {}) {
    this.logger.info(msg, { ...this.boundFields, ...fields });
  }

  error(msg: string, fields: Record<string, any> = {}) {
    this.logger.error(msg, { ...this.boundFields, ...fields });
  }

  warn(msg: string, fields: Record<string, any> = {}) {
    this.logger.warn(msg, { ...this.boundFields, ...fields });
  }

  debug(msg: string, fields: Record<string, any> = {}) {
    this.logger.debug(msg, { ...this.boundFields, ...fields });
  }
}
