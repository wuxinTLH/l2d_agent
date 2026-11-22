import { createClient, commandBody, psdBody, localURL, METHODS } from './transport.mjs'
import { buildEyeRig, buildFaceRig, EYE_RIG_TIMEOUT_MS, FACE_RIG_TIMEOUT_MS } from './eye-rig.mjs'

const renderJSON = (_args, value) => [{ type: 'text', text: JSON.stringify(value, null, 2) }]

const str = description => ({ type: 'string', description })
const integer = description => ({ type: 'integer', description })

/** Pure definitions allow transport tests without installing the DSH peer graph. */
export function createTools(config = {}, fetchImpl) {
  const request = createClient(config, fetchImpl)
  const tool = (name, description, parameters, path, body) => ({
    name, description, parameters, timeoutMs: config.timeoutMs ?? 30_000,
    output: { schema: { type: 'object', additionalProperties: true }, render: renderJSON },
    async execute(args, exec) { return request(path, body(args), exec.signal) },
  })
  return [
    tool('cubism_health', 'Check the local l2d agent studio health. This does not prove Cubism Editor is running, connected or approved.', {}, '/api/health', () => undefined),
    tool('cubism_discover', 'Discover installed Cubism Editor executable candidates through the studio. Read-only; does not launch or select a candidate.', {}, '/api/l2d/cubism/discover', () => ({})),
    tool('cubism_launch', 'Launch the Cubism executable saved in studio configuration. Process launch only: license, readiness and an open model remain unverified.', {}, '/api/l2d/cubism/launch', () => ({})),
    tool('cubism_connect', 'Connect the studio bridge to a loopback Cubism Editor external API WebSocket. Enable the external API in Cubism and approve plugin access there. Token stays server-local. Inspect connected/approved flags; an HTTP success is not approval.', {
      endpoint: str('Optional loopback ws URL with explicit port; default ws://127.0.0.1:22033.'),
    }, '/api/l2d/cubism/connect', args => {
      if (args.endpoint === undefined) return {}
      localURL(args.endpoint, 'ws:')
      return { endpoint: args.endpoint }
    }),
    tool('cubism_command', 'Run only the six supported official document/parameter methods after connect and approval. No arbitrary eval, mesh creation, rigging, model saving or moc3 export. Data uses official PascalCase keys. Parameter writes affect the connected editor; inspect results before retrying.', {
      method: { type: 'string', enum: METHODS, required: true },
      data: { type: 'object', additionalProperties: false, description: 'Method data: empty for GetDocuments/GetCurrentModelUID; GetParameters needs ModelUID or DocumentUID; other methods need ModelUID. Ids optional for GetParameterValues. SetParameterValues needs Parameters.', properties: {
        ModelUID: str('Official model UID'), DocumentUID: str('Official document UID'),
        Ids: { type: 'array', items: str('Parameter Id'), description: 'At most 1024 parameter IDs' },
        Parameters: { type: 'array', description: '1..1024 unique parameter Id/Value pairs', items: { type: 'object', additionalProperties: false, properties: {
          Id: { ...str('Parameter Id'), required: true }, Value: { type: 'number', required: true, description: 'Finite value within +/-1000000' },
        } } },
      } },
    }, '/api/l2d/cubism/command', args => commandBody(args.method, args.data)),
    tool('cubism_disconnect', 'Close the studio Cubism bridge connection. Does not quit Cubism or delete its server-local registration token.', {}, '/api/l2d/cubism/disconnect', () => ({})),
    tool('cubism_export_psd', 'Export existing studio asset IDs to a layered PSD locally (no paid AI call). First layer is topmost; adjacent equal group names form folders. Folders do not set Cubism numeric draw order. No segmentation, mesh or rigging. Returns export id and relative download URL.', {
      width: { ...integer('Canvas width 1..8192; total canvas <=40 megapixels'), required: true },
      height: { ...integer('Canvas height 1..8192'), required: true },
      layers: { type: 'array', required: true, description: '1..64 layers, first topmost; combined pixels <=80 megapixels (server-enforced)', items: { type: 'object', additionalProperties: false, properties: {
        assetId: { ...str('Existing studio asset ID, 24 lowercase hex characters'), required: true },
        name: str('PSD layer name; empty falls back to asset name'), group: str('Optional adjacent folder name'),
        x: integer('Signed pixel offset -8192..8192, defaults to 0'), y: integer('Signed pixel offset -8192..8192, defaults to 0'),
        width: integer('Optional resize width; omit/zero both dimensions to preserve original'), height: integer('Optional resize height; must accompany width'),
      } } },
    }, '/api/oc/export-psd', psdBody),
    tool('cubism_open_psd', 'Forward a studio PSD export to the configured official Cubism document launcher. Launch only, not verified import/model creation. Confirm Model Settings in Cubism; if no dialog appears download the PSD and use File > Open.', {
      exportId: { ...str('Existing studio PSD export ID, 24 lowercase hex characters; no file paths'), required: true },
    }, '/api/l2d/cubism/open-psd', args => {
      if (!/^[a-f0-9]{24}$/.test(args.exportId)) throw new Error('Invalid exportId')
      return { exportId: args.exportId }
    }),
    {
      name: 'cubism_build_eye_rig',
      description: 'Run the fixed Windows desktop auto-mesh and eye keyform recipe in Cubism FREE, then save a new local QA model and screenshots. Only invoke on the user’s explicit request to run this recipe, never as an automatic follow-up. Requires imported test PSD 8b2b9c88e31a0519f6fe2cf3 active in the Editor; do not interact with the desktop during the run. Takes no arguments. Failure or cancellation can leave Editor edits; inspect before retrying.',
      parameters: {},
      timeoutMs: EYE_RIG_TIMEOUT_MS,
      output: { schema: { type: 'object', additionalProperties: false, properties: {
        status: { type: 'string', enum: ['saved'], required: true }, modelPath: { type: 'string', required: true }, screenshots: { type: 'array', required: true, items: { type: 'string' } },
      } }, render: renderJSON },
      async execute(args, exec) { return buildEyeRig(args, exec.signal) },
    },
    {
      name: 'cubism_build_face_rig',
      description: 'Run the fixed Windows desktop face-rig recipe in Cubism FREE for the deterministic layered test fixture only, not an arbitrary character. Builds independent ParamEyeLOpen/ParamEyeROpen keyforms using upper eyelid translation, with separate mouth/tongue parts, then saves a new local QA model and screenshots. Only invoke on the user’s explicit request to run this recipe, never as an automatic follow-up. Requires the imported layered face test fixture active in the Editor; do not interact with the desktop during the run. Takes no arguments. Shares the eye-rig desktop lock. Failure or cancellation can leave Editor edits; inspect before retrying.',
      parameters: {},
      timeoutMs: FACE_RIG_TIMEOUT_MS,
      output: { schema: { type: 'object', additionalProperties: false, properties: {
        status: { type: 'string', enum: ['saved'], required: true }, modelPath: { type: 'string', required: true }, screenshots: { type: 'array', required: true, items: { type: 'string' } },
      } }, render: renderJSON },
      async execute(args, exec) { return buildFaceRig(args, exec.signal) },
    },
  ]
}
