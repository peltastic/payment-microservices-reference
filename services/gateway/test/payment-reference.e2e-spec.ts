import request from 'supertest';

const describeWhenConfigured =
  process.env.PAYMENT_REFERENCE_E2E_BASE_URL &&
  process.env.PAYMENT_REFERENCE_E2E_API_KEY
    ? describe
    : describe.skip;

describeWhenConfigured('Payment Microservices Reference system e2e', () => {
  it('given a valid client payment request when sent through the gateway then creates a payment through the full stack', async () => {
    // Arrange
    const baseURL = process.env.PAYMENT_REFERENCE_E2E_BASE_URL!;
    const apiKey = process.env.PAYMENT_REFERENCE_E2E_API_KEY!;
    const idempotencyKey = `idem_${Date.now()}`;

    // Act
    const response = await request(baseURL)
      .post('/v1/payments')
      .set('Authorization', `Bearer ${apiKey}`)
      .set('Idempotency-Key', idempotencyKey)
      .send({
        amount: 5000,
        customer_email: 'customer@example.com',
        customer_name: 'Customer',
        metadata: { source: 'e2e' },
      });

    // Assert
    expect(response.status).toBe(201);
    expect(response.body).toMatchObject({
      merchant_id: expect.any(String),
      amount: 5000,
      currency: 'NGN',
    });
    expect(response.body.id).toEqual(expect.any(String));
  });

  it('given no api key when a payment request is sent through the gateway then rejects the request', async () => {
    // Arrange
    const baseURL = process.env.PAYMENT_REFERENCE_E2E_BASE_URL!;

    // Act
    const response = await request(baseURL)
      .post('/v1/payments')
      .send({
        amount: 5000,
        customer_email: 'customer@example.com',
        customer_name: 'Customer',
        idempotency_key: `idem_${Date.now()}`,
      });

    // Assert
    expect(response.status).toBe(401);
  });
});
