import {
  BadRequestException,
  Controller,
  Get,
  Post,
  Param,
  Headers,
  Query,
  Req,
} from '@nestjs/common';
import type { Request } from 'express';
import { DeliveriesService } from './deliveries.service';
import { LoggerService } from '../common/filters/logger.service';

type RequestWithContext = Request & {
  requestId?: string;
};

@Controller('deliveries')
export class DeliveriesController {
  constructor(
    private readonly service: DeliveriesService,
    private readonly logger: LoggerService,
  ) {}

  @Get()
  findAll(
    @Headers('x-merchant-id') merchantId: string,
    @Query('page') page = '1',
    @Query('limit') limit = '20',
    @Req() request: RequestWithContext,
  ) {
    const parsedPage = positiveIntegerQuery(page, 'page', 1);
    const parsedLimit = positiveIntegerQuery(limit, 'limit', 20, 100);

    this.log(request, merchantId).info(
      'list webhook deliveries request received',
      {
        page: parsedPage,
        limit: parsedLimit,
      },
    );
    return this.service.findAll(merchantId, parsedPage, parsedLimit);
  }

  @Get('dead-letter')
  deadLetters(
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId).info(
      'list dead letter deliveries request received',
    );
    return this.service.findDeadLetters(merchantId);
  }

  @Post(':id/retry')
  retry(
    @Param('id') id: string,
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId, id).info(
      'retry webhook delivery request received',
    );
    return this.service.retryDelivery(id, merchantId);
  }

  private log(
    request: RequestWithContext,
    merchantId?: string,
    deliveryId?: string,
  ) {
    return this.logger.with({
      component: 'deliveries_controller',
      request_id: request.requestId ?? request.header('x-request-id'),
      merchant_id: merchantId,
      delivery_id: deliveryId,
    });
  }
}

function positiveIntegerQuery(
  value: string | undefined,
  name: string,
  fallback: number,
  max?: number,
): number {
  if (value === undefined || value.trim() === '') {
    return fallback;
  }

  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new BadRequestException({
      error: {
        code: 'validation_error',
        message: `${name} must be a positive integer`,
      },
    });
  }

  if (max !== undefined && parsed > max) {
    throw new BadRequestException({
      error: {
        code: 'validation_error',
        message: `${name} must be less than or equal to ${max}`,
      },
    });
  }

  return parsed;
}
