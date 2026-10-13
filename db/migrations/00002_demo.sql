-- +goose Up
INSERT INTO identity.organizations VALUES('11111111-1111-4111-8111-111111111111','Panlyack Trading','USD','FR_PCG','FIFO');
INSERT INTO cash.accounts VALUES
('11111111-1111-4111-8111-111111111111','CASH',5000),
('11111111-1111-4111-8111-111111111111','BANK',0),
('11111111-1111-4111-8111-111111111111','MOBILE_MONEY',0),
('11111111-1111-4111-8111-111111111111','CARD',0);
INSERT INTO inventory.products(organization_id,id,sku,name,unit,quantity,value) VALUES
('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','OPENING-STOCK','Opening inventory','each',100,2000);
INSERT INTO inventory.stock_lots(organization_id,id,product_id,quantity,unit_cost,source_id) VALUES
('11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333','22222222-2222-4222-8222-222222222222',100,20,'44444444-4444-4444-8444-444444444444');
INSERT INTO inventory.stock_movements(organization_id,id,product_id,quantity,value,source_type,source_id) VALUES
('11111111-1111-4111-8111-111111111111','55555555-5555-4555-8555-555555555555','22222222-2222-4222-8222-222222222222',100,2000,'OPENING','44444444-4444-4444-8444-444444444444');
INSERT INTO cash.movements(organization_id,id,method,amount,source_type,source_id) VALUES
('11111111-1111-4111-8111-111111111111','66666666-6666-4666-8666-666666666666','CASH',5000,'OPENING','44444444-4444-4444-8444-444444444444');
INSERT INTO accounting.accounts(organization_id,role,code,name)
SELECT '11111111-1111-4111-8111-111111111111',role,code,name FROM (VALUES
('CASH','531','Cash'),('BANK','512','Bank'),('MOBILE_MONEY','517','Mobile money'),('CARD','511','Card settlement'),
('INVENTORY','37','Merchandise inventory'),('INPUT_TAX','44566','Deductible input tax'),('OUTPUT_TAX','44571','Collected output tax'),
('ACCOUNTS_PAYABLE','401','Trade payables'),('ACCOUNTS_RECEIVABLE','411','Trade receivables'),('SALES','707','Merchandise sales'),
('COGS','6037','Cost of goods sold'),('EQUITY','101','Opening equity')) AS a(role,code,name);
INSERT INTO accounting.journal_entries(organization_id,id,source_type,source_id,reference,entry_date,currency) VALUES
('11111111-1111-4111-8111-111111111111','77777777-7777-4777-8777-777777777777','OPENING','44444444-4444-4444-8444-444444444444','Opening balances','2026-10-13','USD');
INSERT INTO accounting.journal_lines(organization_id,id,entry_id,role,debit,credit) VALUES
('11111111-1111-4111-8111-111111111111','88888888-8888-4888-8888-888888888881','77777777-7777-4777-8777-777777777777','CASH',5000,0),
('11111111-1111-4111-8111-111111111111','88888888-8888-4888-8888-888888888882','77777777-7777-4777-8777-777777777777','INVENTORY',2000,0),
('11111111-1111-4111-8111-111111111111','88888888-8888-4888-8888-888888888883','77777777-7777-4777-8777-777777777777','EQUITY',0,7000);
INSERT INTO reporting.summary(organization_id,cash,inventory) VALUES('11111111-1111-4111-8111-111111111111',5000,2000);

-- +goose Down
-- Opening accounting entries are immutable. Rebuild an empty demo database instead of deleting posted history.
SELECT 1;
