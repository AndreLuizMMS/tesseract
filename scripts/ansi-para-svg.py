#!/usr/bin/env python3
"""Converte a saída ANSI do Tesseract em SVG, para a vitrine do repositório.

Cada caractere ganha posição própria no eixo x, então a grade continua alinhada
mesmo se a máquina de quem lê não tiver a fonte monoespaçada da pilha. Fundos
contíguos viram um retângulo só.
"""
import re
import sys
import html

LARGURA_CEL = 8.4
ALTURA_LIN = 18.0
MARGEM_X = 16.0
MARGEM_Y = 22.0
FONTE = 14.0
FUNDO = "#070B0C"
FRENTE = "#BFD1C6"
PILHA = "ui-monospace, SFMono-Regular, 'JetBrains Mono', Menlo, Consolas, monospace"

# Caracteres de caixa desenhados como geometria, não como texto: a fonte de
# quem lê não fecha os cantos, e a grade sairia tracejada. Cada entrada diz que
# braços saem do centro da célula e com que peso (1 fino, 2 grosso).
BRACOS = {
    "─": ("lr", 1), "│": ("ud", 1), "┌": ("rd", 1), "┐": ("ld", 1),
    "└": ("ru", 1), "┘": ("lu", 1), "├": ("rud", 1), "┤": ("lud", 1),
    "┬": ("lrd", 1), "┴": ("lru", 1), "┼": ("lrud", 1),
    "━": ("lr", 2), "┃": ("ud", 2), "┏": ("rd", 2), "┓": ("ld", 2),
    "┗": ("ru", 2), "┛": ("lu", 2), "┣": ("rud", 2), "┫": ("lud", 2),
    "┳": ("lrd", 2), "┻": ("lru", 2), "╋": ("lrud", 2),
}
GROSSURA = {1: 1.3, 2: 2.4}


def tracos(linhas):
    """Devolve os retângulos que formam a grade, agrupando o que for vizinho."""
    saida = []
    for y, linha in enumerate(linhas):
        for x, (ch, frente, _, _) in enumerate(linha):
            if ch not in BRACOS:
                continue
            bracos, peso = BRACOS[ch]
            g = GROSSURA[peso]
            cf = frente if frente and frente.startswith("#") else FRENTE
            cx = MARGEM_X + (x + 0.5) * LARGURA_CEL
            cy = MARGEM_Y + y * ALTURA_LIN - FONTE * 0.35
            if "l" in bracos:
                saida.append((cx - LARGURA_CEL / 2, cy - g / 2, LARGURA_CEL / 2 + g / 2, g, cf))
            if "r" in bracos:
                saida.append((cx - g / 2, cy - g / 2, LARGURA_CEL / 2 + g / 2, g, cf))
            if "u" in bracos:
                saida.append((cx - g / 2, cy - ALTURA_LIN / 2, g, ALTURA_LIN / 2 + g / 2, cf))
            if "d" in bracos:
                saida.append((cx - g / 2, cy - g / 2, g, ALTURA_LIN / 2 + g / 2, cf))
    return saida


ESCAPE = re.compile(r"\x1b\[([0-9;]*)m")


def cor(codigo, i):
    """Lê uma cor a partir do parâmetro i de um SGR. Devolve (cor, próximo i)."""
    if codigo[i] in (38, 48) and i + 1 < len(codigo):
        if codigo[i + 1] == 2 and i + 4 < len(codigo):
            r, g, b = codigo[i + 2 : i + 5]
            return f"#{r:02x}{g:02x}{b:02x}", i + 5
        if codigo[i + 1] == 5 and i + 2 < len(codigo):
            return f"ansi{codigo[i + 2]}", i + 3
    return None, i + 1


def celulas(texto):
    """Devolve, por linha, a lista de (caractere, frente, fundo, negrito)."""
    linhas = []
    frente = fundo = None
    negrito = False
    for linha in texto.split("\n"):
        atual = []
        pos = 0
        for m in ESCAPE.finditer(linha):
            for ch in linha[pos : m.start()]:
                atual.append((ch, frente, fundo, negrito))
            codigo = [int(n) if n else 0 for n in (m.group(1) or "0").split(";")]
            i = 0
            while i < len(codigo):
                n = codigo[i]
                if n == 0:
                    frente = fundo = None
                    negrito = False
                    i += 1
                elif n == 1:
                    negrito = True
                    i += 1
                elif n == 38:
                    frente, i = cor(codigo, i)
                elif n == 48:
                    fundo, i = cor(codigo, i)
                elif n == 39:
                    frente, i = None, i + 1
                elif n == 49:
                    fundo, i = None, i + 1
                else:
                    i += 1
            pos = m.end()
        for ch in linha[pos:]:
            atual.append((ch, frente, fundo, negrito))
        linhas.append(atual)
    return linhas


def svg(linhas, titulo):
    colunas = max((len(l) for l in linhas), default=0)
    larg = MARGEM_X * 2 + colunas * LARGURA_CEL
    alt = MARGEM_Y * 2 + len(linhas) * ALTURA_LIN

    partes = [
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {larg:.0f} {alt:.0f}" '
        f'width="{larg:.0f}" height="{alt:.0f}" role="img" aria-label="{html.escape(titulo)}">',
        f"  <title>{html.escape(titulo)}</title>",
        f'  <rect width="{larg:.0f}" height="{alt:.0f}" fill="{FUNDO}"/>',
    ]

    # fundos primeiro, agrupando colunas vizinhas de mesma cor
    for y, linha in enumerate(linhas):
        x = 0
        while x < len(linha):
            f = linha[x][2]
            if not f or not f.startswith("#"):
                x += 1
                continue
            fim = x
            while fim < len(linha) and linha[fim][2] == f:
                fim += 1
            partes.append(
                f'  <rect x="{MARGEM_X + x * LARGURA_CEL:.1f}" y="{MARGEM_Y + y * ALTURA_LIN - FONTE:.1f}" '
                f'width="{(fim - x) * LARGURA_CEL:.1f}" height="{ALTURA_LIN:.1f}" fill="{f}"/>'
            )
            x = fim

    for x, y, w, h, c in tracos(linhas):
        partes.append(
            f'  <rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" fill="{c}"/>'
        )

    partes.append(
        f'  <g font-family="{PILHA}" font-size="{FONTE}" xml:space="preserve">'
    )
    for y, linha in enumerate(linhas):
        x = 0
        while x < len(linha):
            f, n = linha[x][1], linha[x][3]
            fim = x
            while fim < len(linha) and linha[fim][1] == f and linha[fim][3] == n:
                fim += 1
            trecho = "".join(" " if c[0] in BRACOS else c[0] for c in linha[x:fim])
            # O espaço da ponta é jogado fora junto com a coordenada dele: o
            # navegador descarta espaço no começo do texto e, se as posições
            # ficassem, o resto da linha andava para a esquerda.
            recuo = len(trecho) - len(trecho.lstrip(" "))
            trecho = trecho.strip(" ")
            if trecho:
                inicio = x + recuo
                xs = " ".join(
                    f"{MARGEM_X + (inicio + i) * LARGURA_CEL:.1f}" for i in range(len(trecho))
                )
                cf = f if f and f.startswith("#") else FRENTE
                peso = ' font-weight="700"' if n else ""
                partes.append(
                    f'    <text x="{xs}" y="{MARGEM_Y + y * ALTURA_LIN:.1f}" '
                    f'xml:space="preserve" fill="{cf}"{peso}>{html.escape(trecho)}</text>'
                )
            x = fim
    partes.append("  </g>")
    partes.append("</svg>")
    return "\n".join(partes) + "\n"


if __name__ == "__main__":
    entrada, saida, titulo = sys.argv[1], sys.argv[2], sys.argv[3]
    texto = open(entrada, encoding="utf-8").read().rstrip("\n")
    open(saida, "w", encoding="utf-8").write(svg(celulas(texto), titulo))
    print("ok", saida)
