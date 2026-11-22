import Schema from '@deepseek-ai/schemastery'
import { defineTool } from '@deepseek-ai/dsh-tools'
import { createTools } from './tools.mjs'
import { DEFAULT_STUDIO_URL } from './transport.mjs'

export const name = 'cubism'
export const inject = ['tools']
export const Config = Schema.object({
  studioUrl: Schema.string().default(DEFAULT_STUDIO_URL).description('Local studio HTTP origin; loopback only, explicit non-default port, no path/credentials/query.'),
  timeoutMs: Schema.number().min(1).max(600_000).default(30_000).description('Request timeout in milliseconds; mutations are never automatically retried.'),
})

export function apply(ctx, config) {
  for (const definition of createTools(config)) ctx.tools.register(defineTool(definition))
}
