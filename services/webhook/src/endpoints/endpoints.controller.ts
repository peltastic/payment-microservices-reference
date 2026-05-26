import {
  Controller,
  Get,
  Post,
  Put,
  Delete,
  Body,
  Param,
  Headers,
  HttpCode,
  Req,
} from '@nestjs/common';
import type { Request } from 'express';
import { EndpointsService } from './endpoints.service';
import { CreateEndpointDto } from './dto/create-endpoint.dto';
import { UpdateEndpointDto } from './dto/update-endpoint.dto';
import { LoggerService } from '../common/filters/logger.service';

type RequestWithContext = Request & {
  requestId?: string;
};

@Controller('endpoints')
export class EndpointsController {
  constructor(
    private readonly service: EndpointsService,
    private readonly logger: LoggerService,
  ) {}

  @Post()
  create(
    @Body() dto: CreateEndpointDto,
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId).info(
      'create webhook endpoint request received',
      {
        url: dto.url,
        event_count: dto.events?.length ?? 0,
      },
    );
    return this.service.create(merchantId, dto);
  }

  @Get()
  findAll(
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId).info(
      'list webhook endpoints request received',
    );
    return this.service.findAll(merchantId);
  }

  @Put(':id')
  update(
    @Param('id') id: string,
    @Body() dto: UpdateEndpointDto,
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId, id).info(
      'update webhook endpoint request received',
      {
        has_url: dto.url !== undefined,
        has_events: dto.events !== undefined,
        has_description: dto.description !== undefined,
        has_is_active: dto.isActive !== undefined,
      },
    );
    return this.service.update(id, merchantId, dto);
  }

  @Delete(':id')
  @HttpCode(204)
  remove(
    @Param('id') id: string,
    @Headers('x-merchant-id') merchantId: string,
    @Req() request: RequestWithContext,
  ) {
    this.log(request, merchantId, id).info(
      'delete webhook endpoint request received',
    );
    return this.service.remove(id, merchantId);
  }

  private log(
    request: RequestWithContext,
    merchantId: string,
    endpointId?: string,
  ) {
    return this.logger.with({
      component: 'endpoints_controller',
      request_id: request.requestId ?? request.header('x-request-id'),
      merchant_id: merchantId,
      endpoint_id: endpointId,
    });
  }
}
