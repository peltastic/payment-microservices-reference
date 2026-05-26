import { Controller, Get } from '@nestjs/common';
import { AppService } from './app.service';
import { LoggerService } from './common/filters/logger.service';

@Controller()
export class AppController {
  constructor(
    private readonly appService: AppService,
    private readonly logger: LoggerService,
  ) {}

  @Get()
  getHello(): string {
    this.logger
      .with({ component: 'app_controller' })
      .debug('root endpoint requested');
    return this.appService.getHello();
  }

  @Get('health')
  health() {
    this.logger
      .with({ component: 'app_controller' })
      .debug('health endpoint requested');
    return {
      status: 'ok',
      service: 'webhook-service',
      timestamp: new Date().toISOString(),
    };
  }
}
