jest.mock('../endpoints/endpoints.service', () => ({
  EndpointsService: class EndpointsService {},
}));

jest.mock('../deliveries/deliveries.service', () => ({
  DeliveriesService: class DeliveriesService {},
}));

import { EventsConsumer } from './events.consumer';

describe('EventsConsumer error handling', () => {
  const logger = {
    with: jest.fn(() => ({
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    })),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('rejects dispatch when any endpoint delivery cannot be queued', async () => {
    const endpointsService = {
      findActiveForEvent: jest
        .fn()
        .mockResolvedValue([{ id: 'endpoint_ok' }, { id: 'endpoint_failed' }]),
    };
    const deliveriesService = {
      enqueue: jest
        .fn()
        .mockResolvedValueOnce({ id: 'delivery_ok' })
        .mockRejectedValueOnce(new Error('redis unavailable')),
    };
    const consumer = new EventsConsumer(
      endpointsService as never,
      deliveriesService as never,
      logger as never,
    );

    await expect(
      (
        consumer as unknown as {
          dispatch: (event: PaymentEventFixture) => Promise<void>;
        }
      ).dispatch(eventFixture),
    ).rejects.toThrow('failed to enqueue 1 of 2 webhook deliveries');
  });
});

type PaymentEventFixture = typeof eventFixture;

const eventFixture = {
  id: 'evt_test',
  type: 'payment.succeeded',
  version: '1.0',
  timestamp: new Date().toISOString(),
  source: 'payment-service',
  data: {
    payment_id: 'pay_test',
    merchant_id: 'mrc_test',
    amount: 1000,
    currency: 'NGN',
    customer_email: 'customer@example.com',
    customer_name: 'Customer',
    bank_reference: 'bank_ref',
    failure_reason: '',
  },
};
