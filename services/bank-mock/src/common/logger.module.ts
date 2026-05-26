import { Module } from '@nestjs/common';
import { LoggerService } from './filters/logger.service';

@Module({
  providers: [LoggerService],
  exports: [LoggerService],
})
export class LoggerModule {}
