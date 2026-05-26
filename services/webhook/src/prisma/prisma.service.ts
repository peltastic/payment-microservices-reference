import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { PrismaPg } from '@prisma/adapter-pg';
import { LoggerService } from '../common/filters/logger.service';
import { PrismaClient } from '../generated/prisma/client';

@Injectable()
export class PrismaService
  extends PrismaClient
  implements OnModuleInit, OnModuleDestroy
{
  constructor(private readonly loggerService: LoggerService) {
    const connectionString = process.env.DATABASE_URL;
    const logger = loggerService.with({ component: 'prisma' });

    if (!connectionString) {
      logger.error('database url missing');
      throw new Error('DATABASE_URL is required');
    }

    super({
      adapter: new PrismaPg({ connectionString }),
    });
  }

  async onModuleInit() {
    const logger = this.loggerService.with({ component: 'prisma' });

    logger.info('connecting prisma client');

    try {
      await this.$connect();
      logger.info('prisma client connected');
    } catch (error) {
      logger.error('prisma client connection failed', {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  }

  async onModuleDestroy() {
    const logger = this.loggerService.with({ component: 'prisma' });

    logger.info('disconnecting prisma client');
    await this.$disconnect();
    logger.info('prisma client disconnected');
  }
}
