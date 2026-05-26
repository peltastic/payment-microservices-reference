import { Injectable, Scope } from '@nestjs/common';
import winston from 'winston';

@Injectable({ scope: Scope.DEFAULT })
export class LoggerService {
  private readonly serviceName = process.env.OTEL_SERVICE_NAME || 'bank-mock';
  private readonly logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    format: winston.format.json(),
    transports: [new winston.transports.Console()],
  });

  info(msg: string, fields: Record<string, unknown> = {}) {
    this.write('info', msg, fields);
  }

  error(msg: string, fields: Record<string, unknown> = {}) {
    this.write('error', msg, fields);
  }

  warn(msg: string, fields: Record<string, unknown> = {}) {
    this.write('warn', msg, fields);
  }

  debug(msg: string, fields: Record<string, unknown> = {}) {
    if (process.env.LOG_LEVEL !== 'debug') return;
    this.write('debug', msg, fields);
  }

  with(fields: Record<string, unknown>): BoundLogger {
    return new BoundLogger(this, fields);
  }

  private write(level: string, msg: string, fields: Record<string, unknown>) {
    this.logger.log({
      time: new Date().toISOString(),
      env: process.env.ENVIRONMENT || 'development',
      request_id: '',
      merchant_id: '',
      duration_ms: 0,
      component: '',
      trace_id: '',
      span_id: '',
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
    private readonly boundFields: Record<string, unknown>,
  ) {}

  info(msg: string, fields: Record<string, unknown> = {}) {
    this.logger.info(msg, { ...this.boundFields, ...fields });
  }

  error(msg: string, fields: Record<string, unknown> = {}) {
    this.logger.error(msg, { ...this.boundFields, ...fields });
  }

  warn(msg: string, fields: Record<string, unknown> = {}) {
    this.logger.warn(msg, { ...this.boundFields, ...fields });
  }

  debug(msg: string, fields: Record<string, unknown> = {}) {
    this.logger.debug(msg, { ...this.boundFields, ...fields });
  }
}
