-- Transactions always point to a subcategory, so a category without an active subcategory can
-- never be chosen. New categories are created with "Outros"; this gives it to existing ones.

-- +goose Up
INSERT INTO categories (parent_id, name, position)
SELECT c.id, 'Outros', 1
FROM categories c
WHERE c.parent_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM categories s WHERE s.parent_id = c.id AND s.archived = FALSE);

-- +goose Down
SELECT 1;
