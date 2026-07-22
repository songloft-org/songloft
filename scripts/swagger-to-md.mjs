#!/usr/bin/env node
/**
 * swagger-to-md.mjs
 *
 * 读取 docs/swagger.json（Swagger 2.0），按 tag 分组，生成 Markdown 文件到
 * docs/swagger-api/ 目录，供 VitePress 渲染。
 *
 * 用法：node scripts/swagger-to-md.mjs
 *       或在 docs/ 目录下：npm run swagger-to-md
 *
 * 无第三方依赖，纯 Node.js 实现。
 */

import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, '..');
const SWAGGER_PATH = join(ROOT, 'docs', 'swagger.json');
const OUT_DIR = join(ROOT, 'docs', 'swagger-api');

// ─── 工具函数 ────────────────────────────────────────────────────────

/** 将 tag 名称转为文件名友好的 slug */
function slugify(tag) {
  return tag
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9一-鿿-]/g, '');
}

/** tag 排序权重（让通用/认证类排前面） */
const TAG_ORDER = [
  '认证管理', '歌曲管理', '歌单管理', '数据备份', '扫描管理',
  '缓存管理', '配置管理', '设置', '电台与 HLS', '系统管理',
  '系统升级', '资源代理', 'JS插件管理', 'JS 插件',
];

function tagSortKey(tag) {
  const idx = TAG_ORDER.indexOf(tag);
  return idx >= 0 ? idx : TAG_ORDER.length;
}

/** HTTP 方法排序权重 */
const METHOD_ORDER = { get: 0, post: 1, put: 2, patch: 3, delete: 4, head: 5, options: 6 };

/** 从 $ref 提取定义名称 */
function refName(ref) {
  if (!ref) return null;
  return ref.replace('#/definitions/', '');
}

/**
 * 递归解析 schema 为人可读的类型字符串。
 * 对 $ref 返回链接到数据模型页面的 Markdown 链接。
 */
function schemaToType(schema, defs) {
  if (!schema) return 'any';
  if (schema.$ref) {
    const name = refName(schema.$ref);
    const shortName = name.replace(/^(models|handlers|services|jsplugin|source)\./, '');
    return `[${shortName}](./数据模型#${anchorId(name)})`;
  }
  if (schema.type === 'array') {
    const itemType = schema.items ? schemaToType(schema.items, defs) : 'any';
    return `${itemType}[]`;
  }
  if (schema.type === 'object' && schema.additionalProperties) {
    const valType = schemaToType(schema.additionalProperties, defs);
    return `map[string]${valType}`;
  }
  if (schema.type === 'object') return 'object';
  if (schema.type === 'integer') return schema.format === 'int64' ? 'int64' : 'integer';
  if (schema.type === 'number') return schema.format || 'number';
  if (schema.type === 'boolean') return 'boolean';
  if (schema.type === 'string') return schema.format ? `string (${schema.format})` : 'string';
  if (schema.type === 'file') return 'file';
  return schema.type || 'any';
}

/**
 * 转义描述文本中不在反引号内的 HTML 尖括号，防止 VitePress (Vue) 误解析。
 * 反引号内的内容保持原样（Markdown 会渲染为 <code>）。
 */
function escapeAngleBrackets(text) {
  if (!text) return text;
  // 拆分为反引号内外的片段
  const parts = text.split(/(`[^`]*`)/g);
  return parts.map((part, i) => {
    // 奇数索引是反引号包裹的内容，不处理
    if (i % 2 === 1) return part;
    // 偶数索引是普通文本，转义尖括号
    return part.replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }).join('');
}

/** 为定义名称生成锚点 ID */
function anchorId(defName) {
  return defName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
}

/**
 * 将 schema 定义展开为属性表格的 Markdown 行数组。
 * 返回 { lines: string[], hasContent: boolean }
 */
function schemaToPropertyTable(schema, defs, depth = 0) {
  if (!schema) return { lines: [], hasContent: false };

  // 解引用
  if (schema.$ref) {
    const name = refName(schema.$ref);
    const resolved = defs[name];
    if (!resolved) {
      const shortName = name.replace(/^(models|handlers|services|jsplugin|source)\./, '');
      return { lines: [`参见 [${shortName}](./数据模型#${anchorId(name)})`], hasContent: true };
    }
    return schemaToPropertyTable(resolved, defs, depth);
  }

  if (schema.type === 'array' && schema.items) {
    return schemaToPropertyTable(schema.items, defs, depth);
  }

  if (schema.type !== 'object' || !schema.properties) {
    return { lines: [], hasContent: false };
  }

  const required = new Set(schema.required || []);
  const lines = [];
  lines.push('| 字段 | 类型 | 必填 | 说明 |');
  lines.push('|------|------|------|------|');

  for (const [prop, propSchema] of Object.entries(schema.properties)) {
    const type = schemaToType(propSchema, defs);
    const req = required.has(prop) ? '是' : '否';
    let desc = propSchema.description || '';
    if (propSchema.enum) {
      desc += ` 可选值: \`${propSchema.enum.join('`, `')}\``;
    }
    if (propSchema.example !== undefined) {
      desc += ` 示例: \`${JSON.stringify(propSchema.example)}\``;
    }
    if (propSchema.default !== undefined) {
      desc += ` 默认: \`${JSON.stringify(propSchema.default)}\``;
    }
    // 转义管道符和 HTML 尖括号
    desc = escapeAngleBrackets(desc).replace(/\|/g, '\\|').trim();
    lines.push(`| \`${prop}\` | ${type} | ${req} | ${desc} |`);
  }

  return { lines, hasContent: true };
}

// ─── 主流程 ──────────────────────────────────────────────────────────

const swagger = JSON.parse(readFileSync(SWAGGER_PATH, 'utf8'));
const definitions = swagger.definitions || {};
const basePath = swagger.basePath || '';

// 1. 按 tag 分组
const tagGroups = {};   // tag -> [{ method, path, operation }]
const tagDescMap = {};  // tag -> 描述（来自 swagger.tags）

if (swagger.tags) {
  for (const t of swagger.tags) {
    tagDescMap[t.name] = t.description || '';
  }
}

for (const [path, methods] of Object.entries(swagger.paths)) {
  for (const [method, operation] of Object.entries(methods)) {
    if (['parameters', 'summary', 'description'].includes(method)) continue;
    const tag = (operation.tags && operation.tags[0]) || '未分类';
    if (!tagGroups[tag]) tagGroups[tag] = [];
    tagGroups[tag].push({ method: method.toUpperCase(), path, operation });
  }
}

// 排序 tag
const sortedTags = Object.keys(tagGroups).sort((a, b) => tagSortKey(a) - tagSortKey(b));

// 2. 清理并创建输出目录
if (existsSync(OUT_DIR)) {
  rmSync(OUT_DIR, { recursive: true, force: true });
}
mkdirSync(OUT_DIR, { recursive: true });

// 3. 生成索引页
const indexLines = [
  '---',
  'title: API 文档',
  '---',
  '',
  '# Songloft API 文档',
  '',
  `> 基于 Swagger 定义自动生成 | API 版本 ${swagger.info?.version || 'unknown'} | 基础路径 \`${basePath}\``,
  '',
  swagger.info?.description || '',
  '',
  '## 认证方式',
  '',
  '大部分接口需要 Bearer Token 认证。在请求头中添加：',
  '',
  '```',
  'Authorization: Bearer {your_access_token}',
  '```',
  '',
  '通过 [认证管理](./认证管理) 中的登录接口获取 Token。',
  '',
  '## 接口分组',
  '',
  '| 分组 | 接口数 | 说明 |',
  '|------|--------|------|',
];

for (const tag of sortedTags) {
  const slug = slugify(tag);
  const count = tagGroups[tag].length;
  const desc = tagDescMap[tag] || '';
  indexLines.push(`| [${tag}](./${slug}) | ${count} | ${desc} |`);
}

indexLines.push('');
indexLines.push(`| [数据模型](./数据模型) | ${Object.keys(definitions).length} | 请求/响应中使用的数据结构定义 |`);
indexLines.push('');

writeFileSync(join(OUT_DIR, 'index.md'), indexLines.join('\n'), 'utf8');
console.log(`[swagger-to-md] 生成 swagger-api/index.md`);

// 4. 按 tag 生成各分组页面
for (const tag of sortedTags) {
  const entries = tagGroups[tag];
  const slug = slugify(tag);

  // 按路径排序，同路径按方法排序
  entries.sort((a, b) => {
    if (a.path !== b.path) return a.path.localeCompare(b.path);
    return (METHOD_ORDER[a.method.toLowerCase()] || 99) - (METHOD_ORDER[b.method.toLowerCase()] || 99);
  });

  const lines = [
    '---',
    `title: ${tag}`,
    '---',
    '',
    `# ${tag}`,
    '',
  ];

  if (tagDescMap[tag]) {
    lines.push(tagDescMap[tag], '');
  }

  // 接口快速索引
  lines.push('## 接口列表', '');
  lines.push('| 方法 | 路径 | 说明 |');
  lines.push('|------|------|------|');
  for (const { method, path, operation } of entries) {
    const summary = operation.summary || '';
    const anchor = anchorId(`${method}-${path}`);
    lines.push(`| \`${method}\` | [\`${path}\`](#${anchor}) | ${summary} |`);
  }
  lines.push('');

  // 每个接口的详细文档
  for (const { method, path, operation } of entries) {
    const summary = operation.summary || '';
    const description = operation.description || '';
    const needsAuth = !!(operation.security && operation.security.length > 0);

    lines.push(`## ${method} ${path}`, '');
    lines.push(`**${summary}**`, '');
    if (description && description !== summary) {
      lines.push(escapeAngleBrackets(description), '');
    }
    if (needsAuth) {
      lines.push('::: tip 需要认证');
      lines.push('此接口需要 Bearer Token 认证');
      lines.push(':::', '');
    }

    // 参数
    const params = operation.parameters || [];
    const pathParams = params.filter(p => p.in === 'path');
    const queryParams = params.filter(p => p.in === 'query');
    const headerParams = params.filter(p => p.in === 'header');
    const formParams = params.filter(p => p.in === 'formData');
    const bodyParams = params.filter(p => p.in === 'body');

    if (pathParams.length > 0) {
      lines.push('### 路径参数', '');
      lines.push('| 参数 | 类型 | 必填 | 说明 |');
      lines.push('|------|------|------|------|');
      for (const p of pathParams) {
        lines.push(`| \`${p.name}\` | ${p.type || 'string'} | ${p.required ? '是' : '否'} | ${escapeAngleBrackets(p.description || '').replace(/\|/g, '\\|')} |`);
      }
      lines.push('');
    }

    if (queryParams.length > 0) {
      lines.push('### 查询参数', '');
      lines.push('| 参数 | 类型 | 必填 | 说明 |');
      lines.push('|------|------|------|------|');
      for (const p of queryParams) {
        let desc = p.description || '';
        if (p.enum) desc += ` 可选值: \`${p.enum.join('`, `')}\``;
        if (p.default !== undefined) desc += ` 默认: \`${JSON.stringify(p.default)}\``;
        desc = escapeAngleBrackets(desc).replace(/\|/g, '\\|');
        lines.push(`| \`${p.name}\` | ${p.type || 'string'} | ${p.required ? '是' : '否'} | ${desc} |`);
      }
      lines.push('');
    }

    if (headerParams.length > 0) {
      lines.push('### 请求头参数', '');
      lines.push('| 参数 | 类型 | 必填 | 说明 |');
      lines.push('|------|------|------|------|');
      for (const p of headerParams) {
        lines.push(`| \`${p.name}\` | ${p.type || 'string'} | ${p.required ? '是' : '否'} | ${escapeAngleBrackets(p.description || '').replace(/\|/g, '\\|')} |`);
      }
      lines.push('');
    }

    if (formParams.length > 0) {
      lines.push('### 表单参数', '');
      lines.push('| 参数 | 类型 | 必填 | 说明 |');
      lines.push('|------|------|------|------|');
      for (const p of formParams) {
        let desc = p.description || '';
        desc = escapeAngleBrackets(desc).replace(/\|/g, '\\|');
        lines.push(`| \`${p.name}\` | ${p.type || 'string'} | ${p.required ? '是' : '否'} | ${desc} |`);
      }
      lines.push('');
    }

    if (bodyParams.length > 0) {
      lines.push('### 请求体', '');
      for (const p of bodyParams) {
        if (p.description) {
          lines.push(escapeAngleBrackets(p.description), '');
        }
        if (p.schema) {
          const typeStr = schemaToType(p.schema, definitions);
          lines.push(`**类型：** ${typeStr}`, '');

          // 如果有 $ref，展开属性表
          const table = schemaToPropertyTable(p.schema, definitions);
          if (table.hasContent) {
            lines.push(...table.lines, '');
          }

          // 如果是数组且 items 有 $ref，也展开
          if (p.schema.type === 'array' && p.schema.items?.$ref) {
            const itemTable = schemaToPropertyTable(p.schema.items, definitions);
            if (itemTable.hasContent) {
              lines.push('数组元素结构：', '');
              lines.push(...itemTable.lines, '');
            }
          }
        }
      }
    }

    // 响应
    const responses = operation.responses || {};
    const responseCodes = Object.keys(responses).sort();
    if (responseCodes.length > 0) {
      lines.push('### 响应', '');
      for (const code of responseCodes) {
        const resp = responses[code];
        lines.push(`#### ${code} - ${escapeAngleBrackets(resp.description || '')}`, '');
        if (resp.schema) {
          const typeStr = schemaToType(resp.schema, definitions);
          lines.push(`**类型：** ${typeStr}`, '');

          const table = schemaToPropertyTable(resp.schema, definitions);
          if (table.hasContent) {
            lines.push(...table.lines, '');
          }

          // 如果是数组且 items 有 $ref
          if (resp.schema.type === 'array' && resp.schema.items?.$ref) {
            const itemTable = schemaToPropertyTable(resp.schema.items, definitions);
            if (itemTable.hasContent) {
              lines.push('数组元素结构：', '');
              lines.push(...itemTable.lines, '');
            }
          }
        }
      }
    }

    // Content-Type 信息
    const consumes = operation.consumes || [];
    const produces = operation.produces || [];
    if (consumes.length > 0 || produces.length > 0) {
      lines.push('### 内容类型', '');
      if (consumes.length > 0) {
        lines.push(`- **请求：** \`${consumes.join('`, `')}\``);
      }
      if (produces.length > 0) {
        lines.push(`- **响应：** \`${produces.join('`, `')}\``);
      }
      lines.push('');
    }

    lines.push('---', '');
  }

  const filePath = join(OUT_DIR, `${slug}.md`);
  writeFileSync(filePath, lines.join('\n'), 'utf8');
  console.log(`[swagger-to-md] 生成 swagger-api/${slug}.md (${entries.length} 个接口)`);
}

// 5. 生成数据模型页面
{
  const lines = [
    '---',
    'title: 数据模型',
    '---',
    '',
    '# 数据模型',
    '',
    '> API 请求和响应中使用的数据结构定义。',
    '',
    '## 模型索引',
    '',
    '| 模型 | 说明 |',
    '|------|------|',
  ];

  // 按类别分组排序
  const sortedDefs = Object.keys(definitions).sort((a, b) => {
    // 先按前缀分组
    const prefixA = a.split('.')[0];
    const prefixB = b.split('.')[0];
    if (prefixA !== prefixB) return prefixA.localeCompare(prefixB);
    return a.localeCompare(b);
  });

  for (const name of sortedDefs) {
    const def = definitions[name];
    const shortName = name.replace(/^(models|handlers|services|jsplugin|source)\./, '');
    const desc = getDefDescription(def);
    lines.push(`| [\`${shortName}\`](#${anchorId(name)}) | ${desc} |`);
  }
  lines.push('');

  // 每个定义的详细文档
  for (const name of sortedDefs) {
    const def = definitions[name];
    const shortName = name.replace(/^(models|handlers|services|jsplugin|source)\./, '');

    lines.push(`## ${shortName} {#${anchorId(name)}}`, '');
    lines.push(`完整名称：\`${name}\``, '');

    if (def.type) {
      lines.push(`**类型：** \`${def.type}\``, '');
    }

    if (def.description) {
      lines.push(escapeAngleBrackets(def.description), '');
    }

    if (def.type === 'object' && def.properties) {
      const required = new Set(def.required || []);
      lines.push('| 字段 | 类型 | 必填 | 说明 |');
      lines.push('|------|------|------|------|');

      for (const [prop, propSchema] of Object.entries(def.properties)) {
        const type = schemaToType(propSchema, definitions);
        const req = required.has(prop) ? '是' : '否';
        let desc = propSchema.description || '';
        if (propSchema.enum) {
          desc += ` 可选值: \`${propSchema.enum.join('`, `')}\``;
        }
        if (propSchema.example !== undefined) {
          const exStr = JSON.stringify(propSchema.example);
          // 只在示例值不过长时展示
          if (exStr.length < 80) {
            desc += ` 示例: \`${exStr}\``;
          }
        }
        if (propSchema.default !== undefined) {
          desc += ` 默认: \`${JSON.stringify(propSchema.default)}\``;
        }
        desc = escapeAngleBrackets(desc).replace(/\|/g, '\\|').trim();
        lines.push(`| \`${prop}\` | ${type} | ${req} | ${desc} |`);
      }
      lines.push('');
    } else if (def.type === 'array' && def.items) {
      const itemType = schemaToType(def.items, definitions);
      lines.push(`数组元素类型：${itemType}`, '');
    } else if (def.enum) {
      lines.push('可选值：', '');
      for (const v of def.enum) {
        lines.push(`- \`${v}\``);
      }
      lines.push('');
    }

    lines.push('---', '');
  }

  writeFileSync(join(OUT_DIR, '数据模型.md'), lines.join('\n'), 'utf8');
  console.log(`[swagger-to-md] 生成 swagger-api/数据模型.md (${sortedDefs.length} 个模型)`);
}

console.log(`[swagger-to-md] 完成！共生成 ${sortedTags.length + 2} 个文件`);

// ─── 辅助 ────────────────────────────────────────────────────────────

/** 从定义中提取简短描述（取第一个属性的 description 或返回类型信息） */
function getDefDescription(def) {
  if (def.description) return escapeAngleBrackets(def.description).replace(/\|/g, '\\|');
  if (def.type === 'object' && def.properties) {
    const props = Object.keys(def.properties);
    if (props.length > 0) {
      return `包含 ${props.length} 个字段`;
    }
  }
  return def.type || '';
}
