import { Module } from '@nestjs/common';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { BankModule } from './bank/bank.module';
import { LoggerModule } from './common/logger.module';
import { RequestIdInterceptor } from './common/filters/request-id.interceptor';

@Module({
  imports: [LoggerModule, BankModule],
  controllers: [AppController],
  providers: [
    AppService,
    {
      provide: APP_INTERCEPTOR,
      useClass: RequestIdInterceptor,
    },
  ],
})
export class AppModule {}
