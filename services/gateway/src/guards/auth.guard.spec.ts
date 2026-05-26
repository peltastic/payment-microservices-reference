import {
  ExecutionContext,
  ServiceUnavailableException,
  UnauthorizedException,
} from '@nestjs/common';
import { throwError } from 'rxjs';
import { AuthGuard } from './auth.guard';

describe('AuthGuard error handling', () => {
  const logger = {
    with: jest.fn(() => ({
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    })),
  };
  const validationsTotal = { inc: jest.fn() };

  beforeEach(() => {
    jest.clearAllMocks();
    process.env.AUTH_SERVICE_URL = 'http://auth:4001/api/v1/auth';
    process.env.INTERNAL_AUTH_SECRET = 'test-secret';
  });

  it('returns service unavailable when auth service cannot be reached', async () => {
    const httpService = {
      post: jest.fn(() =>
        throwError(() => ({
          isAxiosError: true,
          message: 'timeout of 5000ms exceeded',
        })),
      ),
    };
    const guard = new AuthGuard(
      httpService as never,
      logger as never,
      validationsTotal as never,
    );

    await expect(guard.canActivate(context())).rejects.toBeInstanceOf(
      ServiceUnavailableException,
    );
    expect(validationsTotal.inc).toHaveBeenCalledWith({
      result: 'unavailable',
    });
  });

  it('keeps invalid key failures as unauthorized', async () => {
    const httpService = {
      post: jest.fn(() =>
        throwError(() => ({
          isAxiosError: true,
          response: {
            status: 401,
            data: { error: { code: 'invalid_key' } },
          },
        })),
      ),
    };
    const guard = new AuthGuard(
      httpService as never,
      logger as never,
      validationsTotal as never,
    );

    await expect(guard.canActivate(context())).rejects.toBeInstanceOf(
      UnauthorizedException,
    );
    expect(validationsTotal.inc).toHaveBeenCalledWith({ result: 'invalid' });
  });
});

function context(): ExecutionContext {
  const request = {
    headers: { authorization: 'Bearer pk_test' },
    header: jest.fn((name: string) =>
      name.toLowerCase() === 'x-request-id' ? 'req_test' : undefined,
    ),
    method: 'GET',
    originalUrl: '/v1/payments',
    url: '/v1/payments',
  };
  const response = {
    setHeader: jest.fn(),
  };

  return {
    switchToHttp: () => ({
      getRequest: () => request,
      getResponse: () => response,
    }),
  } as unknown as ExecutionContext;
}
