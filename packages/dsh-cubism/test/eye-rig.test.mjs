import test from 'node:test'
import assert from 'node:assert/strict'
import { once } from 'node:events'
import { mkdtemp, mkdir, realpath, rm, symlink, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, win32 } from 'node:path'
import { createEyeRigRunner, EYE_RIG_TIMEOUT_MS, FACE_RIG_TIMEOUT_MS, verifyEyeRigOutput } from '../src/eye-rig.mjs'
import { createTools } from '../src/tools.mjs'

async function fixture(t, recipe = 'eye') {
  const workspace = await mkdtemp(join(await realpath(tmpdir()), 'cubism-eye-rig-'))
  t.after(() => rm(workspace, { recursive: true, force: true }))
  const root = join(workspace, 'packages', 'dsh-cubism')
  const directory = join(workspace, '.local', 'qa', recipe === 'eye' ? 'one-click-rig' : 'face-rig')
  const script = join(root, 'scripts', `build-${recipe}-rig.ps1`)
  await mkdir(join(root, 'scripts'), { recursive: true })
  await writeFile(script, '# test placeholder; never executed')
  return { root, directory, script, recipe, systemRoot: 'C:\\Windows', platform: 'win32' }
}

async function writeResult(directory) {
  const result = { status: 'saved', modelPath: join(directory, 'model.cmo3'), screenshots: [join(directory, 'proof.png')] }
  await Promise.all([result.modelPath, ...result.screenshots].map(file => writeFile(file, 'nonempty')))
  return result
}

test('eye rig is an explicit no-argument non-HTTP tool with fixed timeout', async () => {
  const tool = createTools({ timeoutMs: 1 }, () => assert.fail('Unexpected fetch')).find(tool => tool.name === 'cubism_build_eye_rig')
  assert.deepEqual(tool.parameters, {})
  assert.equal(tool.timeoutMs, EYE_RIG_TIMEOUT_MS)
  await assert.rejects(tool.execute({ script: 'override' }, {}), /takes no arguments/)
})

test('runner invokes only the fixed package script and returns only verified outputs', async t => {
  const opts = await fixture(t)
  const run = createEyeRigRunner({ ...opts, executeFile: async (program, args, options) => {
    assert.equal(program, win32.join(opts.systemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe'))
    assert.deepEqual(args, ['-NoLogo', '-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', opts.script, '-OutputDirectory', opts.directory])
    assert.equal(options.cwd, opts.root)
    assert.equal(options.shell, false)
    assert.ok(options.signal instanceof AbortSignal)
    return { stdout: '\uFEFF' + JSON.stringify({ ...await writeResult(opts.directory), extra: 'not returned' }) }
  } })
  assert.deepEqual(await run({}), { status: 'saved', modelPath: join(opts.directory, 'model.cmo3'), screenshots: [join(opts.directory, 'proof.png')] })
})

test('verification rejects malformed, out-of-root, empty and missing artifacts', async t => {
  const { directory, root } = await fixture(t)
  await mkdir(directory, { recursive: true })
  const result = await writeResult(directory)
  const empty = join(directory, 'empty.png')
  const outside = join(root, 'outside.cmo3')
  await writeFile(empty, '')
  await writeFile(outside, 'nonempty')
  for (const invalid of ['secret stderr', JSON.stringify({ ...result, modelPath: outside }), JSON.stringify({ ...result, screenshots: [] }), JSON.stringify({ ...result, screenshots: [empty] }), JSON.stringify({ ...result, screenshots: [join(directory, 'absent.png')] })]) {
    await assert.rejects(verifyEyeRigOutput(directory, invalid), /^Error: Eye-rig script did not return verified model and screenshot files\./)
  }
})

test('runner rejects overlap across instances, propagates cancellation, and sanitizes process failure', async t => {
  const opts = await fixture(t)
  const abort = new AbortController()
  const started = Promise.withResolvers()
  let launches = 0
  const run = createEyeRigRunner({ ...opts, executeFile: async (_program, _args, { signal }) => {
    launches++
    started.resolve()
    await once(signal, 'abort')
    throw new Error('secret stderr')
  } })
  const fail = createEyeRigRunner({ ...opts, executeFile: async () => { throw new Error('secret stderr') } })
  const pending = run({}, abort.signal)
  await started.promise
  await assert.rejects(fail({}), /already holds the QA lock/)
  abort.abort(new Error('secret cancellation reason'))
  await assert.rejects(pending, /^Error: Eye-rig cancelled\./)
  await assert.rejects(fail({}), /^Error: Eye-rig automation failed\./)
  await assert.rejects(run({}, abort.signal), /cancelled/)
  assert.equal(launches, 1)
})

test('runner rejects an output junction before writing outside the workspace', async t => {
  const opts = await fixture(t)
  const local = join(opts.root, '..', '..', '.local')
  await symlink(opts.root, local, 'junction')
  const run = createEyeRigRunner({ ...opts, executeFile: async () => assert.fail('Unexpected spawn') })
  await assert.rejects(run({}), /QA output directory is unavailable/)
})

test('face rig is a fixed no-argument non-HTTP deterministic fixture tool', async t => {
  const tool = createTools({ timeoutMs: 1 }, () => assert.fail('Unexpected fetch')).find(tool => tool.name === 'cubism_build_face_rig')
  assert.deepEqual(tool.parameters, {})
  assert.equal(tool.timeoutMs, FACE_RIG_TIMEOUT_MS)
  for (const text of ['deterministic layered test fixture', 'not an arbitrary character', 'ParamEyeLOpen/ParamEyeROpen', 'upper eyelid translation', 'separate mouth/tongue']) assert.ok(tool.description.includes(text))
  for (const args of [undefined, null, [], { script: 'override' }, { output: 'override' }, { recipe: 'eye' }]) await assert.rejects(tool.execute(args, {}), /takes no arguments/)
  const opts = await fixture(t, 'face')
  const run = createEyeRigRunner({ ...opts, executeFile: async (program, args, options) => {
    assert.equal(program, win32.join(opts.systemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe'))
    assert.deepEqual(args, ['-NoLogo', '-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', opts.script, '-OutputDirectory', opts.directory])
    assert.equal(options.shell, false)
    assert.equal(options.cwd, opts.root)
    return { stdout: JSON.stringify(await writeResult(opts.directory)) }
  } })
  assert.equal((await run({})).modelPath, join(opts.directory, 'model.cmo3'))
  assert.throws(() => createEyeRigRunner({ recipe: '../outside' }), /Unknown fixed/)
})

test('eye and face recipes exclude each other in both directions and release the common lock', async t => {
  const opts = await fixture(t)
  await writeFile(join(opts.root, 'scripts', 'build-face-rig.ps1'), '# test placeholder; never executed')
  for (const recipe of ['eye', 'face']) {
    const abort = new AbortController()
    const started = Promise.withResolvers()
    const run = createEyeRigRunner({ ...opts, recipe, executeFile: async (_program, _args, { signal }) => {
      started.resolve()
      await once(signal, 'abort')
      throw new Error('secret stderr')
    } })
    const other = createEyeRigRunner({ ...opts, recipe: recipe === 'eye' ? 'face' : 'eye', executeFile: async () => { throw new Error('secret stderr') } })
    const pending = run({}, abort.signal)
    await started.promise
    await assert.rejects(other({}), /already holds the QA lock/)
    abort.abort()
    await assert.rejects(pending, /cancelled/)
    await assert.rejects(other({}), /automation failed/)
  }
})

test('face runner rejects redirected QA paths and invalid output without spawning a desktop', async t => {
  const opts = await fixture(t, 'face')
  await mkdir(opts.directory, { recursive: true })
  const invalid = createEyeRigRunner({ ...opts, executeFile: async () => ({ stdout: 'secret stderr' }) })
  await assert.rejects(invalid({}), /^Error: Face-rig script did not return verified/)
  await rm(opts.directory, { recursive: true })
  await symlink(opts.root, opts.directory, 'junction')
  const run = createEyeRigRunner({ ...opts, executeFile: async () => assert.fail('Unexpected spawn') })
  await assert.rejects(run({}), /QA output directory is unavailable/)
})

test('runner aborts the subprocess at deadline', async t => {
  const opts = await fixture(t)
  const keepAlive = setInterval(() => {}, 1000)
  t.after(() => clearInterval(keepAlive))
  const run = createEyeRigRunner({ ...opts, timeoutMs: 100, executeFile: async (_program, _args, { signal }) => {
    await once(signal, 'abort')
    throw new Error('secret stderr')
  } })
  await assert.rejects(run({}), /^Error: Eye-rig timed out\./)
})
