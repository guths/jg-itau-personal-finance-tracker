#!/usr/bin/env python3
"""Gera os PDFs sintéticos dos testes de pdftext (sem nenhum dado real).

Requer PyMuPDF (pip install pymupdf). Rodar dentro de testdata/:
    python3 gen_fixtures.py
"""
import fitz


def build():
    doc = fitz.open()
    page = doc.new_page(width=595.28, height=841.89)
    page.insert_text((72, 100), "Topo Lançamentos: compras", fontsize=9, fontname="helv")
    page.insert_text((72, 120), "28/08 PADARIA EXEMPLO", fontsize=9, fontname="helv")
    page.insert_text((300, 120), "12,50", fontsize=9, fontname="helv")
    page.insert_text((72, 800), "Rodapé", fontsize=9, fontname="helv")
    return doc


build().save("plain.pdf")
build().save(
    "encrypted.pdf",
    encryption=fitz.PDF_ENCRYPT_RC4_128,
    owner_pw="dono",
    user_pw="teste123",
    permissions=fitz.PDF_PERM_PRINT | fitz.PDF_PERM_COPY,
)
