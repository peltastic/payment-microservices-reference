import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { LoggerService } from './common/filters/logger.service';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { logger: false });
  const logger = app.get(LoggerService).with({ component: 'bootstrap' });

  const port = process.env.PORT ?? 5001;
  logger.info('starting bank mock service', { port });
  await app.listen(port);
  logger.info('bank mock service started', { port });
}

bootstrap().catch((error: unknown) => {
  new LoggerService()
    .with({ component: 'bootstrap' })
    .error('bank mock service failed to start', {
      error: error instanceof Error ? error.message : String(error),
    });
  process.exit(1);
});
