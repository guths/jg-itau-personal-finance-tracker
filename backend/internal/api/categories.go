package api

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

type subcategoryJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	Archived bool   `json:"archived"`
}

type categoryJSON struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Color         string            `json:"color"`
	Position      int               `json:"position"`
	Archived      bool              `json:"archived"`
	Subcategories []subcategoryJSON `json:"subcategories"`
}

// getCategories returns the category tree (archived items included, flagged).
func (s *Server) getCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, COALESCE(parent_id, 0), name, COALESCE(color, ''), position, archived
		FROM categories ORDER BY parent_id IS NOT NULL, position, id`)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	defer rows.Close()
	out := []categoryJSON{}
	index := map[int64]int{}
	for rows.Next() {
		var id, parent int64
		var name, color string
		var pos int
		var archived bool
		if err := rows.Scan(&id, &parent, &name, &color, &pos, &archived); err != nil {
			writeInternal(w, r, err)
			return
		}
		if parent == 0 {
			index[id] = len(out)
			out = append(out, categoryJSON{ID: id, Name: name, Color: color, Position: pos, Archived: archived, Subcategories: []subcategoryJSON{}})
			continue
		}
		if i, ok := index[parent]; ok {
			out[i].Subcategories = append(out[i].Subcategories, subcategoryJSON{ID: id, Name: name, Position: pos, Archived: archived})
		}
	}
	if err := rows.Err(); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// postCategory creates a category ({name, color}) or a subcategory ({name, parent_id}).
func (s *Server) postCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parent_id"`
		Color    string `json:"color"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len([]rune(name)) > 64 {
		writeError(w, http.StatusUnprocessableEntity, "o nome deve ter entre 1 e 64 caracteres")
		return
	}
	ctx := r.Context()
	var parent, color any
	if body.ParentID != nil {
		var pp sql.NullInt64
		err := s.store.DB.QueryRowContext(ctx, `SELECT parent_id FROM categories WHERE id = ?`, *body.ParentID).Scan(&pp)
		if errors.Is(err, sql.ErrNoRows) || pp.Valid {
			writeError(w, http.StatusUnprocessableEntity, "parent_id deve ser uma categoria (não subcategoria)")
			return
		}
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		parent = *body.ParentID
	} else {
		if !colorRe.MatchString(body.Color) {
			writeError(w, http.StatusUnprocessableEntity, "cor inválida: use #RRGGBB")
			return
		}
		color = body.Color
	}
	res, err := s.store.DB.ExecContext(ctx, `INSERT INTO categories (parent_id, name, color, position)
		SELECT ?, ?, ?, COALESCE(MAX(position), 0) + 1 FROM categories WHERE parent_id <=> ?`, parent, name, color, parent)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// patchCategory renames, recolors, reorders or (un)archives a category or subcategory.
func (s *Server) patchCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     *string `json:"name"`
		Color    *string `json:"color"`
		Position *int    `json:"position"`
		Archived *bool   `json:"archived"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	var parent sql.NullInt64
	err := s.store.DB.QueryRowContext(ctx, `SELECT parent_id FROM categories WHERE id = ?`, id).Scan(&parent)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "categoria não encontrada")
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	var sets []string
	var args []any
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len([]rune(name)) > 64 {
			writeError(w, http.StatusUnprocessableEntity, "o nome deve ter entre 1 e 64 caracteres")
			return
		}
		sets, args = append(sets, "name = ?"), append(args, name)
	}
	if body.Color != nil {
		if parent.Valid {
			writeError(w, http.StatusUnprocessableEntity, "subcategorias não têm cor")
			return
		}
		if !colorRe.MatchString(*body.Color) {
			writeError(w, http.StatusUnprocessableEntity, "cor inválida: use #RRGGBB")
			return
		}
		sets, args = append(sets, "color = ?"), append(args, *body.Color)
	}
	if body.Position != nil {
		sets, args = append(sets, "position = ?"), append(args, *body.Position)
	}
	if body.Archived != nil {
		sets, args = append(sets, "archived = ?"), append(args, *body.Archived)
	}
	if len(sets) == 0 {
		writeError(w, http.StatusBadRequest, "nada para alterar")
		return
	}
	if _, err := s.store.DB.ExecContext(ctx, `UPDATE categories SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, id)...); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}
