import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import type { Request } from 'express';
import { createHash, createHmac, timingSafeEqual } from 'node:crypto';

type InternalRequest = Request & {
  requestId?: string;
  merchantId?: string;
};

const TIMESTAMP_HEADER = 'x-internal-timestamp';
const SIGNATURE_HEADER = 'x-internal-signature';
const BODY_HASH_HEADER = 'x-internal-body-sha256';
const MAX_CLOCK_SKEW_SECONDS = 300;

@Injectable()
export class InternalAuthGuard implements CanActivate {
  canActivate(context: ExecutionContext): boolean {
    const request = context.switchToHttp().getRequest<InternalRequest>();
    const path = (request.originalUrl || request.url).split('?')[0];

    if (path === '/health' || path === '/metrics') {
      return true;
    }

    const secret = process.env.INTERNAL_AUTH_SECRET;
    const timestamp = request.header(TIMESTAMP_HEADER) ?? '';
    const signature = request.header(SIGNATURE_HEADER) ?? '';
    const providedBodyHash = request.header(BODY_HASH_HEADER) ?? '';
    const actualBodyHash = this.bodySha256(request);
    const requestId = request.header('x-request-id') ?? '';
    const merchantId = request.header('x-merchant-id') ?? '';

    if (
      !secret ||
      !this.isFreshTimestamp(timestamp) ||
      !this.sameDigest(providedBodyHash, actualBodyHash) ||
      !this.validSignature({
        secret,
        signature,
        method: request.method,
        pathWithQuery: request.originalUrl || request.url,
        timestamp,
        requestId,
        merchantId,
        bodySha256: actualBodyHash,
      })
    ) {
      throw new UnauthorizedException({
        error: {
          code: 'invalid_internal_signature',
          message: 'Internal authentication failed',
        },
      });
    }

    request.requestId = requestId;
    request.merchantId = merchantId;
    return true;
  }

  private bodySha256(request: Request): string {
    const body = (request as Request & { body?: unknown }).body;
    const hasBodyHeaders =
      request.headers['content-length'] !== undefined ||
      request.headers['transfer-encoding'] !== undefined;

    if (!hasBodyHeaders && this.emptyBody(body)) {
      return createHash('sha256').update('').digest('hex');
    }

    if (body === undefined || body === null) {
      return createHash('sha256').update('').digest('hex');
    }

    if (Buffer.isBuffer(body)) {
      return createHash('sha256').update(body).digest('hex');
    }

    if (typeof body === 'string') {
      return createHash('sha256').update(body).digest('hex');
    }

    return createHash('sha256').update(JSON.stringify(body)).digest('hex');
  }

  private emptyBody(body: unknown): boolean {
    return (
      body === undefined ||
      body === null ||
      (typeof body === 'object' &&
        !Buffer.isBuffer(body) &&
        Object.keys(body).length === 0)
    );
  }

  private isFreshTimestamp(value: string): boolean {
    const timestamp = Number(value);

    if (!Number.isFinite(timestamp)) {
      return false;
    }

    return (
      Math.abs(Math.floor(Date.now() / 1000) - timestamp) <=
      MAX_CLOCK_SKEW_SECONDS
    );
  }

  private validSignature(input: {
    secret: string;
    signature: string;
    method: string;
    pathWithQuery: string;
    timestamp: string;
    requestId: string;
    merchantId: string;
    bodySha256: string;
  }): boolean {
    const canonical = [
      input.method.toUpperCase(),
      input.pathWithQuery,
      input.timestamp,
      input.requestId,
      input.merchantId,
      input.bodySha256,
    ].join('\n');
    const expected = createHmac('sha256', input.secret)
      .update(canonical)
      .digest('hex');
    const provided = Buffer.from(input.signature);
    const calculated = Buffer.from(expected);

    return (
      provided.length === calculated.length &&
      timingSafeEqual(provided, calculated)
    );
  }

  private sameDigest(
    providedDigest: string,
    calculatedDigest: string,
  ): boolean {
    const provided = Buffer.from(providedDigest);
    const calculated = Buffer.from(calculatedDigest);

    return (
      provided.length === calculated.length &&
      timingSafeEqual(provided, calculated)
    );
  }
}
