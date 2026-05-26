import {
  CanActivate,
  ExecutionContext,
  HttpException,
  HttpStatus,
  Injectable,
} from '@nestjs/common';
import { createHash } from 'node:crypto';
import type { Request, Response } from 'express';

type GatewayRequest = Request & {
  merchantId?: string;
};

type Bucket = {
  count: number;
  resetAt: number;
};

@Injectable()
export class RateLimitGuard implements CanActivate {
  private readonly buckets = new Map<string, Bucket>();
  private readonly windowMs = Number(
    process.env.RATE_LIMIT_WINDOW_MS || 60_000,
  );
  private readonly maxRequests = Number(process.env.RATE_LIMIT_MAX || 120);

  canActivate(context: ExecutionContext): boolean {
    const http = context.switchToHttp();
    const request = http.getRequest<GatewayRequest>();
    const response = http.getResponse<Response>();
    const path = (request.originalUrl || request.url).split('?')[0];

    if (
      path === '/health' ||
      (path === '/metrics' && this.isInternalClient(request))
    ) {
      return true;
    }

    const key = this.bucketKey(request);
    const now = Date.now();
    const bucket = this.currentBucket(key, now);
    const remaining = Math.max(this.maxRequests - bucket.count - 1, 0);

    response.setHeader('x-ratelimit-limit', String(this.maxRequests));
    response.setHeader('x-ratelimit-remaining', String(remaining));
    response.setHeader(
      'x-ratelimit-reset',
      String(Math.ceil(bucket.resetAt / 1000)),
    );

    if (bucket.count >= this.maxRequests) {
      throw new HttpException(
        {
          error: {
            code: 'rate_limited',
            message: 'Too many requests',
          },
        },
        HttpStatus.TOO_MANY_REQUESTS,
      );
    }

    bucket.count += 1;
    return true;
  }

  private currentBucket(key: string, now: number): Bucket {
    const existing = this.buckets.get(key);

    if (existing && existing.resetAt > now) {
      return existing;
    }

    const bucket = {
      count: 0,
      resetAt: now + this.windowMs,
    };
    this.buckets.set(key, bucket);
    this.prune(now);
    return bucket;
  }

  private bucketKey(request: GatewayRequest): string {
    if (request.merchantId) {
      return `merchant:${request.merchantId}`;
    }

    const authorization = request.header('authorization');
    if (authorization) {
      return `auth:${this.hash(authorization)}`;
    }

    return `ip:${request.ip || request.socket.remoteAddress || 'unknown'}`;
  }

  private prune(now: number) {
    if (this.buckets.size < 10_000) {
      return;
    }

    for (const [key, bucket] of this.buckets.entries()) {
      if (bucket.resetAt <= now) {
        this.buckets.delete(key);
      }
    }
  }

  private hash(value: string): string {
    return createHash('sha256').update(value).digest('hex').slice(0, 16);
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
}
