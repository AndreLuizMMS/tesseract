-- themes/nvim/tesseract.lua
-- Tesseract Neon colorscheme for Neovim.
--
-- Install:
--   cp themes/nvim/tesseract.lua ~/.config/nvim/colors/tesseract.lua
--   :colorscheme tesseract
--
-- Rules inherited from the palette:
--   * phosphor green (#55FFA6) appears ONCE per screen — here it's the cursor;
--   * string and diff-add use the dark green brand.core, which repeats
--     without competing with the cursor;
--   * cyan is structure: window separator, title, popup frame;
--   * diagnostics never use green or cyan — error/warning/info/hint come
--     from the state colors (dead, block, read, muted).
--
-- Fallback: each group declares ctermfg/ctermbg alongside the hex, so the
-- theme stays legible in a 16-color terminal. With NO_COLOR=1 the file
-- loads in monochrome mode (only bold, italic, reverse and underline).

vim.cmd("highlight clear")
if vim.fn.exists("syntax_on") == 1 then
  vim.cmd("syntax reset")
end

vim.o.background = "dark"
vim.g.colors_name = "tesseract"

local sem_cor = (vim.env.NO_COLOR ~= nil and vim.env.NO_COLOR ~= "")

-- Palette tokens. No loose hex below this table.
local c = {
  bg_void     = "#030507",
  bg_base     = "#070B0C",
  bg_surface  = "#0C1315",
  bg_raised   = "#121C1F",
  line_dim    = "#16282A",
  line_active = "#205047",

  fg_faint    = "#3E534E",
  fg_muted    = "#6C8076",
  fg_default  = "#BFD1C6",
  fg_bright   = "#E8F4EC",

  brand_deep     = "#0B3322",
  brand_core     = "#1F7A4C",
  brand_live     = "#35C27A",
  brand_phosphor = "#55FFA6",

  flux_deep = "#082F31",
  flux_core = "#128C86",
  flux      = "#22E0D0",

  st_working = "#6C8076",
  st_read    = "#7DB7E8",
  st_block   = "#FFB454",
  st_dead    = "#FF3B47",
  st_off     = "#3E534E",
  st_orphan  = "#C77DFF",

  ansi_yellow = "#C9A227",
  ansi_purple = "#8B4FC4",
  ansi_red    = "#C22F38",
  ansi_blue   = "#3E7FA8",
}

-- 16-color equivalent of each token, for terminals without 24-bit support.
local ct = {
  [c.bg_void] = 0, [c.bg_base] = 0, [c.bg_surface] = 0, [c.bg_raised] = 8,
  [c.line_dim] = 8, [c.line_active] = 2,
  [c.fg_faint] = 8, [c.fg_muted] = 8, [c.fg_default] = 7, [c.fg_bright] = 15,
  [c.brand_deep] = 2, [c.brand_core] = 2, [c.brand_live] = 2, [c.brand_phosphor] = 10,
  [c.flux_deep] = 6, [c.flux_core] = 6, [c.flux] = 14,
  [c.st_read] = 12, [c.st_block] = 11, [c.st_dead] = 9, [c.st_orphan] = 13,
  [c.ansi_yellow] = 3, [c.ansi_purple] = 5, [c.ansi_red] = 1, [c.ansi_blue] = 4,
}

local function hl(grupo, spec)
  local def = {
    bold = spec.bold,
    italic = spec.italic,
    underline = spec.underline,
    reverse = spec.reverse,
  }
  if not sem_cor then
    def.fg = spec.fg
    def.bg = spec.bg
    def.sp = spec.sp
    def.ctermfg = spec.fg and ct[spec.fg] or nil
    def.ctermbg = spec.bg and ct[spec.bg] or nil
  end
  vim.api.nvim_set_hl(0, grupo, def)
end

-- Surface ----------------------------------------------------------------
hl("Normal",        { fg = c.fg_default, bg = c.bg_base })
hl("NormalFloat",   { fg = c.fg_default, bg = c.bg_surface })
hl("FloatBorder",   { fg = c.flux_core,  bg = c.bg_surface })
hl("FloatTitle",    { fg = c.flux,       bg = c.bg_surface, bold = true })
hl("CursorLine",    { bg = c.bg_surface })
hl("CursorColumn",  { bg = c.bg_surface })
hl("CursorLineNr",  { fg = c.fg_bright,  bg = c.bg_surface, bold = true })
hl("LineNr",        { fg = c.fg_faint })
hl("SignColumn",    { bg = c.bg_base })
hl("ColorColumn",   { bg = c.bg_surface })
hl("Visual",        { bg = c.bg_raised, fg = c.fg_bright })
hl("VisualNOS",     { bg = c.bg_raised })
hl("Cursor",        { fg = c.bg_void, bg = c.brand_phosphor })
hl("lCursor",       { fg = c.bg_void, bg = c.brand_phosphor })
hl("TermCursor",    { fg = c.bg_void, bg = c.brand_phosphor })
hl("MatchParen",    { fg = c.flux, bold = true })
hl("NonText",       { fg = c.fg_faint })
hl("Whitespace",    { fg = c.line_dim })
hl("EndOfBuffer",   { fg = c.bg_base })
hl("Folded",        { fg = c.fg_muted, bg = c.bg_surface })
hl("FoldColumn",    { fg = c.fg_faint, bg = c.bg_base })
hl("Conceal",       { fg = c.fg_faint })
hl("Directory",     { fg = c.flux_core, bold = true })
hl("Title",         { fg = c.fg_bright, bold = true })
hl("Question",      { fg = c.st_block })
hl("MoreMsg",       { fg = c.fg_muted })
hl("ModeMsg",       { fg = c.fg_bright, bold = true })
hl("ErrorMsg",      { fg = c.bg_void, bg = c.st_dead, bold = true })
hl("WarningMsg",    { fg = c.st_block, bold = true })

-- Structure ----------------------------------------------------------------
hl("WinSeparator",  { fg = c.line_dim })
hl("VertSplit",     { fg = c.line_dim })
hl("StatusLine",    { fg = c.fg_bright, bg = c.bg_raised })
hl("StatusLineNC",  { fg = c.fg_muted,  bg = c.bg_surface })
hl("TabLine",       { fg = c.fg_muted,  bg = c.bg_surface })
hl("TabLineFill",   { bg = c.bg_base })
hl("TabLineSel",    { fg = c.fg_bright, bg = c.bg_raised, bold = true })
hl("WinBar",        { fg = c.flux_core, bg = c.bg_base, bold = true })
hl("WinBarNC",      { fg = c.fg_faint,  bg = c.bg_base })

-- Syntax ---------------------------------------------------------------------
hl("Comment",       { fg = c.fg_faint, italic = true })
hl("String",        { fg = c.brand_core })
hl("Character",     { fg = c.brand_core })
hl("Function",      { fg = c.st_read })
hl("Identifier",    { fg = c.fg_default })
hl("Keyword",       { fg = c.st_orphan })
hl("Statement",     { fg = c.st_orphan })
hl("Conditional",   { fg = c.st_orphan })
hl("Repeat",        { fg = c.st_orphan })
hl("Operator",      { fg = c.fg_muted })
hl("PreProc",       { fg = c.flux })
hl("Include",       { fg = c.flux })
hl("Type",          { fg = c.ansi_yellow })
hl("StorageClass",  { fg = c.ansi_yellow })
hl("Structure",     { fg = c.ansi_yellow })
hl("Constant",      { fg = c.st_block })
hl("Number",        { fg = c.st_block })
hl("Boolean",       { fg = c.st_block })
hl("Float",         { fg = c.st_block })
hl("Special",       { fg = c.flux })
hl("SpecialKey",    { fg = c.fg_faint })
hl("Delimiter",     { fg = c.fg_muted })
hl("Todo",          { fg = c.bg_void, bg = c.st_block, bold = true })
hl("Error",         { fg = c.bg_void, bg = c.st_dead, bold = true })
hl("Underlined",    { fg = c.flux, underline = true })

-- Treesitter (only what doesn't inherit from the groups above).
hl("@variable",         { fg = c.fg_default })
hl("@variable.builtin", { fg = c.ansi_purple })
hl("@property",         { fg = c.fg_default })
hl("@field",            { fg = c.fg_default })
hl("@parameter",        { fg = c.fg_muted })
hl("@constructor",      { fg = c.ansi_yellow })
hl("@punctuation",      { fg = c.fg_muted })
hl("@string.escape",    { fg = c.flux })
hl("@string.regexp",    { fg = c.flux })
hl("@tag",              { fg = c.st_orphan })
hl("@tag.attribute",    { fg = c.flux })

-- Diagnostics — no green, no cyan --------------------------------------------
hl("DiagnosticError", { fg = c.st_dead })
hl("DiagnosticWarn",  { fg = c.st_block })
hl("DiagnosticInfo",  { fg = c.st_read })
hl("DiagnosticHint",  { fg = c.fg_muted })
hl("DiagnosticOk",    { fg = c.fg_default })

hl("DiagnosticUnderlineError", { sp = c.st_dead,  underline = true })
hl("DiagnosticUnderlineWarn",  { sp = c.st_block, underline = true })
hl("DiagnosticUnderlineInfo",  { sp = c.st_read,  underline = true })
hl("DiagnosticUnderlineHint",  { sp = c.fg_muted, underline = true })

hl("DiagnosticVirtualTextError", { fg = c.st_dead,  bg = c.bg_surface })
hl("DiagnosticVirtualTextWarn",  { fg = c.st_block, bg = c.bg_surface })
hl("DiagnosticVirtualTextInfo",  { fg = c.st_read,  bg = c.bg_surface })
hl("DiagnosticVirtualTextHint",  { fg = c.fg_muted, bg = c.bg_surface })

hl("DiagnosticSignError", { fg = c.st_dead })
hl("DiagnosticSignWarn",  { fg = c.st_block })
hl("DiagnosticSignInfo",  { fg = c.st_read })
hl("DiagnosticSignHint",  { fg = c.fg_muted })

-- Diff -------------------------------------------------------------------
hl("DiffAdd",     { fg = c.brand_core, bg = c.bg_surface })
hl("DiffChange",  { fg = c.st_block,   bg = c.bg_surface })
hl("DiffDelete",  { fg = c.st_dead,    bg = c.bg_surface })
hl("DiffText",    { fg = c.bg_void,    bg = c.st_block, bold = true })
hl("Added",       { fg = c.brand_core })
hl("Changed",     { fg = c.st_block })
hl("Removed",     { fg = c.st_dead })

-- Search -----------------------------------------------------------------
hl("Search",     { fg = c.bg_void, bg = c.flux })
hl("IncSearch",  { fg = c.bg_void, bg = c.st_block, bold = true })
hl("CurSearch",  { fg = c.bg_void, bg = c.st_block, bold = true })
hl("Substitute", { fg = c.bg_void, bg = c.st_block })

-- Menu -------------------------------------------------------------------
hl("Pmenu",       { fg = c.fg_default, bg = c.bg_surface })
hl("PmenuSel",    { fg = c.fg_bright,  bg = c.bg_raised, bold = true })
hl("PmenuSbar",   { bg = c.bg_raised })
hl("PmenuThumb",  { bg = c.flux_core })
hl("PmenuKind",   { fg = c.flux_core,  bg = c.bg_surface })
hl("PmenuExtra",  { fg = c.fg_muted,   bg = c.bg_surface })
hl("WildMenu",    { fg = c.bg_void,    bg = c.flux })

-- Embedded terminal: the same ANSI 16 as the rest of the theme.
vim.g.terminal_color_0  = c.bg_base
vim.g.terminal_color_1  = c.ansi_red
vim.g.terminal_color_2  = c.brand_core
vim.g.terminal_color_3  = c.ansi_yellow
vim.g.terminal_color_4  = c.ansi_blue
vim.g.terminal_color_5  = c.ansi_purple
vim.g.terminal_color_6  = c.flux_core
vim.g.terminal_color_7  = c.fg_default
vim.g.terminal_color_8  = c.fg_faint
vim.g.terminal_color_9  = c.st_dead
vim.g.terminal_color_10 = c.brand_phosphor
vim.g.terminal_color_11 = c.st_block
vim.g.terminal_color_12 = c.st_read
vim.g.terminal_color_13 = c.st_orphan
vim.g.terminal_color_14 = c.flux
vim.g.terminal_color_15 = c.fg_bright
