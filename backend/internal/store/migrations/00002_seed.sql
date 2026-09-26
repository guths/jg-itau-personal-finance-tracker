-- Initial taxonomy (spec §15) and seeded rules (spec §9.2).
-- Category colors: fixed 13-color categorical palette readable on light and dark themes;
-- subcategories have no color.

-- +goose Up
INSERT INTO categories (id, parent_id, name, color, position) VALUES
  (1, NULL, 'Moradia', '#4E79A7', 1),
  (2, NULL, 'Alimentação', '#F28E2B', 2),
  (3, NULL, 'Transporte', '#E15759', 3),
  (4, NULL, 'Saúde', '#76B7B2', 4),
  (5, NULL, 'Lazer e esporte', '#59A14F', 5),
  (6, NULL, 'Compras', '#EDC948', 6),
  (7, NULL, 'Assinaturas', '#B07AA1', 7),
  (8, NULL, 'Cuidados pessoais', '#FF9DA7', 8),
  (9, NULL, 'Educação', '#9C755F', 9),
  (10, NULL, 'Viagens', '#17BECF', 10),
  (11, NULL, 'Pessoas e presentes', '#BCBD22', 11),
  (12, NULL, 'Taxas', '#7B6FD6', 12),
  (13, NULL, 'Outros', '#8C8C8C', 13);

INSERT INTO categories (parent_id, name, position) VALUES
  (1, 'Aluguel', 1),
  (1, 'Condomínio', 2),
  (1, 'Energia', 3),
  (1, 'Água e gás', 4),
  (1, 'Celular e internet', 5),
  (1, 'Manutenção', 6),
  (1, 'Outros', 7),
  (2, 'Supermercado', 1),
  (2, 'Restaurante', 2),
  (2, 'Delivery', 3),
  (2, 'Padaria e café', 4),
  (2, 'Outros', 5),
  (3, 'Aplicativo', 1),
  (3, 'Combustível', 2),
  (3, 'Estacionamento e pedágio', 3),
  (3, 'Manutenção do veículo', 4),
  (3, 'Outros', 5),
  (4, 'Terapia', 1),
  (4, 'Farmácia', 2),
  (4, 'Consultas e exames', 3),
  (4, 'Plano de saúde', 4),
  (4, 'Outros', 5),
  (5, 'Padel', 1),
  (5, 'Academia', 2),
  (5, 'Bares e eventos', 3),
  (5, 'Ingressos e shows', 4),
  (5, 'Games', 5),
  (5, 'Outros', 6),
  (6, 'Roupas', 1),
  (6, 'Eletrônicos', 2),
  (6, 'Casa', 3),
  (6, 'Online', 4),
  (6, 'Outros', 5),
  (7, 'Software e IA', 1),
  (7, 'Streaming', 2),
  (7, 'Outros', 3),
  (8, 'Barbearia', 1),
  (8, 'Estética', 2),
  (8, 'Outros', 3),
  (9, 'Cursos', 1),
  (9, 'Livros', 2),
  (9, 'Outros', 3),
  (10, 'Passagens', 1),
  (10, 'Hospedagem', 2),
  (10, 'Passeios', 3),
  (10, 'Outros', 4),
  (11, 'Presentes', 1),
  (11, 'Ajuda familiar', 2),
  (11, 'Divisão de contas', 3),
  (11, 'Outros', 4),
  (12, 'IOF', 1),
  (12, 'Tarifas e anuidade', 2),
  (12, 'Juros', 3),
  (12, 'Outros', 4),
  (13, 'Outros', 1);

INSERT INTO rules (merchant_key, category_id, mode, source)
  SELECT 'REPASSE DE IOF EM R$', s.id, 'always', 'user' FROM categories s JOIN categories c ON s.parent_id = c.id
  WHERE c.name = 'Taxas' AND s.name = 'IOF';
INSERT INTO rules (merchant_key, category_id, mode, source)
  SELECT 'MENSALIDADE - PLANO DO', s.id, 'always', 'user' FROM categories s JOIN categories c ON s.parent_id = c.id
  WHERE c.name = 'Taxas' AND s.name = 'Tarifas e anuidade';
INSERT INTO rules (merchant_key, category_id, mode, source)
  SELECT 'REDUCAO MENSALIDADE - P', s.id, 'always', 'user' FROM categories s JOIN categories c ON s.parent_id = c.id
  WHERE c.name = 'Taxas' AND s.name = 'Tarifas e anuidade';

-- +goose Down
DELETE FROM rules;
DELETE FROM categories WHERE parent_id IS NOT NULL;
DELETE FROM categories;
