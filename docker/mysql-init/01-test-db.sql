-- Bancos dos testes de integração (MYSQL_TEST_DSN). Cada pacote Go de teste usa o seu próprio
-- banco finance_test_<pacote> (criado pelo storetest), então o grant cobre o prefixo.
CREATE DATABASE IF NOT EXISTS finance_test CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
GRANT ALL PRIVILEGES ON `finance\_test%`.* TO 'finance'@'%';
FLUSH PRIVILEGES;
