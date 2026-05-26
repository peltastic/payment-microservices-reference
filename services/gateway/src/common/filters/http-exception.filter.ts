import {
  ArgumentsHost,
  Catch,
  ExceptionFilter,
  HttpException,
  HttpStatus,
  Injectable,
} from '@nestjs/common';
import type { Request, Response } from 'express';
import { LoggerService } from './logger.service';

type GatewayRequest = Request & {
  requestId?: string;
  merchantId?: string;
};

type ErrorPayload = {
  error: {
    code: string;
    message: string | string[];
    request_id: string;
    timestamp: string;
  };
};

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function normalizeExceptionResponse(response: unknown): {
  code: string;
  message: string | string[];
} {
  if (typeof response === 'string') {
    return { code: response, message: response };
  }

  if (!isObject(response)) {
    return { code: 'internal_error', message: 'An error occurred' };
  }

  const code =
    typeof response.error === 'string'
      ? response.error
      : isObject(response.error) && typeof response.error.code === 'string'
        ? response.error.code
        : 'error';

  const message =
    typeof response.message === 'string' || Array.isArray(response.message)
      ? response.message
      : isObject(response.error) && typeof response.error.message === 'string'
        ? response.error.message
        : 'An error occurred';

  return { code, message };
}

@Injectable()
@Catch()
export class HttpExceptionFilter implements ExceptionFilter {
  constructor(private readonly logger: LoggerService) {}

  catch(exception: unknown, host: ArgumentsHost) {
    const ctx = host.switchToHttp();
    const response = ctx.getResponse<Response>();
    const request = ctx.getRequest<GatewayRequest>();

    const status =
      exception instanceof HttpException
        ? exception.getStatus()
        : HttpStatus.INTERNAL_SERVER_ERROR;

    const message =
      exception instanceof HttpException
        ? exception.getResponse()
        : 'internal_error';
    const normalized = normalizeExceptionResponse(message);
    const requestId = request.requestId ?? request.header('x-request-id') ?? '';
    const merchantId = request.merchantId ?? request.header('x-merchant-id') ?? '';

    const body: ErrorPayload = {
      error: {
        code: normalized.code,
        message: normalized.message,
        request_id: requestId,
        timestamp: new Date().toISOString(),
      },
    };

    this.logger
      .with({
        component: 'http_exception',
        request_id: requestId,
        merchant_id: merchantId,
        method: request.method,
        path: request.url,
        status,
      })
      .error('request failed', {
        error_code: body.error.code,
        error: body.error.message,
      });

    response.status(status).json(body);
  }
}