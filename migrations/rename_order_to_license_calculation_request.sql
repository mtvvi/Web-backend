-- Миграция для удаления старой колонки order_id
-- Дата: 2025-12-26
-- Цель: Удалить старую колонку order_id, т.к. уже есть license_calculation_request_id

-- 1. Удаляем внешний ключ для старой колонки order_id
ALTER TABLE license_payment_request_services 
DROP CONSTRAINT IF EXISTS fk_license_payment_request_services_order;

-- 2. Удаляем индекс для старой колонки order_id
DROP INDEX IF EXISTS idx_license_payment_request_services_order_id;

-- 3. Удаляем саму колонку order_id
ALTER TABLE license_payment_request_services 
DROP COLUMN IF EXISTS order_id;

-- 4. Проверка результата
SELECT 
    column_name, 
    data_type, 
    is_nullable,
    column_default
FROM information_schema.columns
WHERE table_name = 'license_payment_request_services'
ORDER BY ordinal_position;
