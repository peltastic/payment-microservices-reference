import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { App } from 'supertest/types';
import { AppController } from '../src/app.controller';
import { AppService } from '../src/app.service';
import { LoggerService } from '../src/common/filters/logger.service';

describe('AppController (e2e)', () => {
  let app: INestApplication<App>;

  beforeEach(async () => {
    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [AppController],
      providers: [AppService, LoggerService],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  it('given a health request when the webhook app is running then returns healthy status', () => {
    return request(app.getHttpServer())
      .get('/health')
      .expect(200)
      .expect((response) => {
        expect(response.body).toMatchObject({
          status: 'ok',
          service: 'webhook-service',
        });
      });
  });

  afterEach(async () => {
    await app.close();
  });
});
