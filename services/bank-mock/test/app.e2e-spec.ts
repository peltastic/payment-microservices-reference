import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { App } from 'supertest/types';
import { AppModule } from './../src/app.module';

describe('AppController (e2e)', () => {
  let app: INestApplication<App>;

  beforeEach(async () => {
    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  it('given a root request when the bank mock app is running then returns service greeting', () => {
    return request(app.getHttpServer())
      .get('/')
      .expect(200)
      .expect('Hello World!');
  });

  it('given a bank void request when the bank mock app is running then returns the voided reference', () => {
    return request(app.getHttpServer())
      .post('/bank/void')
      .send({ reference: 'bank_ref_e2e' })
      .expect(201)
      .expect({ success: true, voided: 'bank_ref_e2e' });
  });

  afterEach(async () => {
    await app.close();
  });
});
