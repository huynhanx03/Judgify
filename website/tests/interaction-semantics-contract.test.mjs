import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();
const SOURCE_ROOTS = ["app", "components", "modules"];
const NON_SEMANTIC_TAGS = new Set(["div", "li", "p", "span"]);
const RAW_PALETTE_UTILITY = /(?:bg|text|border|from|to|via|shadow)-(?:blue|sky|indigo|violet|purple|pink|rose|red|orange|yellow|amber|lime|green|emerald|teal|cyan|zinc|neutral|stone|slate|gray)-\d{2,3}(?:\/[\d.]+)?/g;

function filesIn(relativePath) {
  const root = path.join(ROOT, relativePath);
  const entries = fs.readdirSync(root, { withFileTypes: true });
  return entries.flatMap((entry) => {
    const child = path.join(relativePath, entry.name);
    if (entry.isDirectory()) return filesIn(child);
    return entry.name.endsWith(".tsx") ? [child] : [];
  });
}

function clickHandlersOnNonSemanticElements(relativePath) {
  const source = ts.createSourceFile(
    relativePath,
    fs.readFileSync(path.join(ROOT, relativePath), "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const violations = [];
  const visit = (node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(source);
      const hasClick = node.attributes.properties.some(
        (attribute) =>
          ts.isJsxAttribute(attribute) && attribute.name.text === "onClick",
      );
      if (hasClick && NON_SEMANTIC_TAGS.has(tag)) {
        const position = source.getLineAndCharacterOfPosition(node.getStart(source));
        violations.push(`${relativePath}:${position.line + 1} <${tag} onClick>`);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return violations;
}

function imagesWithoutAlternativeText(relativePath) {
  const source = ts.createSourceFile(
    relativePath,
    fs.readFileSync(path.join(ROOT, relativePath), "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const violations = [];
  const visit = (node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(source);
      if (tag === "Image" || tag === "img") {
        const hasAlternativeText = node.attributes.properties.some(
          (attribute) =>
            ts.isJsxAttribute(attribute) && attribute.name.text === "alt",
        );
        if (!hasAlternativeText) {
          const position = source.getLineAndCharacterOfPosition(node.getStart(source));
          violations.push(`${relativePath}:${position.line + 1} <${tag}>`);
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return violations;
}

function fillImagesWithoutResponsiveSizes(relativePath) {
  const source = ts.createSourceFile(
    relativePath,
    fs.readFileSync(path.join(ROOT, relativePath), "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const violations = [];
  const visit = (node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(source);
      if (tag === "Image") {
        const attributes = node.attributes.properties;
        const fillsContainer = attributes.some(
          (attribute) =>
            ts.isJsxAttribute(attribute) && attribute.name.text === "fill",
        );
        const hasSizes = attributes.some(
          (attribute) =>
            ts.isJsxAttribute(attribute) && attribute.name.text === "sizes",
        );
        if (fillsContainer && !hasSizes) {
          const position = source.getLineAndCharacterOfPosition(node.getStart(source));
          violations.push(`${relativePath}:${position.line + 1} <Image fill>`);
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return violations;
}

function rawPaletteUtilities(relativePath) {
  const contents = fs.readFileSync(path.join(ROOT, relativePath), "utf8");
  return [...contents.matchAll(RAW_PALETTE_UTILITY)].map((match) => {
    const line = contents.slice(0, match.index).split("\n").length;
    return `${relativePath}:${line} ${match[0]}`;
  });
}

test("interactive product controls use semantic elements instead of mouse-only containers", () => {
  const violations = SOURCE_ROOTS.flatMap(filesIn).flatMap(clickHandlersOnNonSemanticElements);
  assert.deepEqual(violations, []);
});

test("product images declare alternative text or an explicit decorative alt", () => {
  const violations = SOURCE_ROOTS.flatMap(filesIn).flatMap(imagesWithoutAlternativeText);
  assert.deepEqual(violations, []);
});

test("fill images declare responsive sizes to avoid over-downloading", () => {
  const violations = SOURCE_ROOTS.flatMap(filesIn).flatMap(fillImagesWithoutResponsiveSizes);
  assert.deepEqual(violations, []);
});

test("product surfaces use semantic color roles instead of raw palette utilities", () => {
  const violations = SOURCE_ROOTS.flatMap(filesIn).flatMap(rawPaletteUtilities);
  assert.deepEqual(violations, []);
});
