import { expect, test } from 'claude-code/testing'

import { clampTop, cut, moveBy, wrapRows, wrappedRows } from '../hooks/list'

test('janela: o selecionado fica à vista, movendo o mínimo', () => {
  expect(clampTop(0, 0, 3, 10)).toBe(0)
  expect(clampTop(0, 2, 3, 10)).toBe(0)
  expect(clampTop(0, 3, 3, 10)).toBe(1)
  expect(clampTop(5, 2, 3, 10)).toBe(2)
  expect(clampTop(9, 9, 3, 10)).toBe(7)
  // Lista menor que a janela nunca rola.
  expect(clampTop(4, 1, 8, 3)).toBe(0)
  // Uma linha só: a janela é o selecionado.
  expect(clampTop(0, 4, 1, 10)).toBe(4)
  expect(clampTop(0, 4, 0, 10)).toBe(4)
})

test('janela: andar para além da borda leva a janela, e para nos extremos', () => {
  expect(moveBy({ selected: 2, top: 0 }, 1, 3, 10)).toEqual({ selected: 3, top: 1 })
  expect(moveBy({ selected: 1, top: 1 }, -1, 3, 10)).toEqual({ selected: 0, top: 0 })
  expect(moveBy({ selected: 9, top: 7 }, 1, 3, 10)).toEqual({ selected: 9, top: 7 })
  expect(moveBy({ selected: 0, top: 0 }, -1, 3, 10)).toEqual({ selected: 0, top: 0 })
  expect(moveBy({ selected: 0, top: 0 }, 5, 3, 10)).toEqual({ selected: 5, top: 3 })
  expect(moveBy({ selected: 3, top: 2 }, 1, 3, 0)).toEqual({ selected: 0, top: 0 })
})

test('largura: fileira que quebra, corte com reticências, texto que dobra', () => {
  expect(wrapRows([], 2, 30)).toBe(0)
  expect(wrapRows([5, 7, 10, 6], 2, 76)).toBe(1)
  expect(wrapRows([5, 7, 10, 6], 2, 30)).toBe(2)
  expect(wrapRows([40], 2, 30)).toBe(1)
  expect(cut('api', 8)).toBe('api')
  expect(cut('a-very-long-service', 8)).toBe('a-very-…')
  expect(cut('abc', 1)).toBe('a')
  expect(wrappedRows('x'.repeat(61), 30)).toBe(3)
  expect(wrappedRows('', 30)).toBe(1)
})
