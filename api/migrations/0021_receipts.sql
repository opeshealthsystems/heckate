-- Each recorded payment gets a numbered receipt (HS-RCT) so the client can be handed a verifiable proof of payment.
ALTER TABLE sales.payments ADD COLUMN receipt_ref text UNIQUE;
