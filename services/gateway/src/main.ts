// This import MUST be before everything else
// Node.js loads modules in order — auto-instrumentation patches modules as they load
import './telemetry/tracer';

import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { LoggerService } from './common/filters/logger.service';
import { MetricsInterceptor } from './interceptors/metrics.interceptor';

async function bootstrap() {
  validateRequiredEnv([
    'AUTH_SERVICE_URL',
    'PAYMENT_SERVICE_URL',
    'LEDGER_SERVICE_URL',
    'WEBHOOK_SERVICE_URL',
    'INTERNAL_AUTH_SECRET',
  ]);
  validateProductionSecret('INTERNAL_AUTH_SECRET');
  const app = await NestFactory.create(AppModule, { logger: false });
  const logger = app.get(LoggerService).with({ component: 'bootstrap' });

  const port = process.env.PORT ?? 3000;
  logger.info('starting gateway service', { port });
  app.useGlobalInterceptors(app.get(MetricsInterceptor));
  await app.listen(port);
  logger.info('gateway service started', { port });
}

function validateProductionSecret(key: string) {
  if (!isProduction()) return;

  const value = process.env[key]?.trim() ?? '';
  if (value.length < 32) {
    throw new Error(`${key} must be at least 32 characters in production`);
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
    .error('gateway service failed to start', {
      error: error instanceof Error ? error.message : String(error),
    });
  process.exit(1);
});
