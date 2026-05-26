// This import MUST be before everything else
// Node.js loads modules in order — auto-instrumentation patches modules as they load
import './telemetry/tracer';

import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
import { AppModule } from './app.module';
import { LoggerService } from './common/filters/logger.service';

async function bootstrap() {
  validateRequiredEnv([
    'DATABASE_URL',
    'EVENT_SIGNING_SECRET',
    'INTERNAL_AUTH_SECRET',
    'WEBHOOK_SECRET_ENCRYPTION_KEY',
  ]);
  validateProductionSecret('EVENT_SIGNING_SECRET');
  validateProductionSecret('INTERNAL_AUTH_SECRET');
  validateProductionEncryptionKey('WEBHOOK_SECRET_ENCRYPTION_KEY');
  const app = await NestFactory.create(AppModule, {
    logger: false, // disable NestJS default logger — we use structured JSON
  });
  const logger = app.get(LoggerService).with({ component: 'bootstrap' });

  app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));

  const port = process.env.PORT || 3000;
  logger.info('starting webhook service', { port });
  await app.listen(port);

  logger.info('webhook service started', { port });
}

function validateProductionSecret(key: string) {
  if (!isProduction()) return;

  const value = process.env[key]?.trim() ?? '';
  if (value.length < 32) {
    throw new Error(`${key} must be at least 32 characters in production`);
  }
}

function validateProductionEncryptionKey(key: string) {
  if (!isProduction()) return;

  const value = process.env[key]?.trim() ?? '';
  const isHexKey = /^[a-f0-9]{64}$/i.test(value);
  const isBase64Key = Buffer.from(value, 'base64').length === 32;

  if (!isHexKey && !isBase64Key) {
    throw new Error(
      `${key} must be a 32-byte key encoded as 64 hex characters or base64 in production`,
    );
  }
}

function isProduction(): boolean {
  return (
    process.env.NODE_ENV === 'production' ||
    process.env.ENVIRONMENT === 'production'
  );
}

function validateRequiredEnv(keys: string[]) {
  const missing = keys.filter((key) => !process.env[key]?.trim()).sort();
  if (missing.length > 0) {
    throw new Error(
      `Missing required environment variables: ${missing.join(', ')}`,
    );
  }
}

bootstrap().catch((error: unknown) => {
  new LoggerService()
    .with({ component: 'bootstrap' })
    .error('webhook service failed to start', {
      error: error instanceof Error ? error.message : String(error),
    });
  process.exit(1);
});
