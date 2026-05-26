import { Global, Module } from '@nestjs/common'
import { LoggerService } from './filters/logger.service'

@Global()
@Module({
  providers: [LoggerService],
  exports: [LoggerService],
})
export class LoggerModule {}