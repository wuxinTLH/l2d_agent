import { execFile } from 'node:child_process'
import { lstat, mkdir, open, realpath, unlink } from 'node:fs/promises'
import { extname, isAbsolute, relative, resolve, sep, win32 } from 'node:path'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'

export const EYE_RIG_TIMEOUT_MS = 90_000
export const FACE_RIG_TIMEOUT_MS = EYE_RIG_TIMEOUT_MS
const recipes = Object.freeze({
  eye: { script: 'build-eye-rig.ps1', output: 'one-click-rig', label: 'Eye-rig', name: 'eye-rig' },
  face: { script: 'build-face-rig.ps1', output: 'face-rig', label: 'Face-rig', name: 'face-rig' },
})
const packageRoot = fileURLToPath(new URL('../', import.meta.url))
const execFileAsync = promisify(execFile)

async function runFile(...args) {
  const pending = execFileAsync(...args)
  // execFile can reject on abort before the child has closed its handles.
  const closed = new Promise(done => pending.child.once('close', done))
  try { return await pending } finally { await closed }
}

function isInside(directory, path) {
  const rel = relative(directory, path)
  return rel !== '' && rel !== '..' && !rel.startsWith(`..${sep}`) && !isAbsolute(rel)
}

async function verifiedFile(directory, value, extension) {
  if (typeof value !== 'string' || !isAbsolute(value) || extname(value).toLowerCase() !== extension || !isInside(directory, value)) throw new Error()
  const file = await realpath(value)
  const st = await lstat(value)
  if (!isInside(directory, file) || !st.isFile() || st.size === 0) throw new Error()
  return file
}

export async function verifyEyeRigOutput(directory, stdout, label = 'Eye-rig') {
  try {
    const data = JSON.parse(stdout.replace(/^\uFEFF/, ''))
    if (data?.status !== 'saved' || !Array.isArray(data.screenshots) || !data.screenshots.length || data.screenshots.length > 16) throw new Error()
    const modelPath = await verifiedFile(directory, data.modelPath, '.cmo3')
    const screenshots = await Promise.all(data.screenshots.map(file => verifiedFile(directory, file, '.png')))
    if (new Set(screenshots).size !== screenshots.length) throw new Error()
    return { status: 'saved', modelPath, screenshots }
  } catch {
    throw new Error(`${label} script did not return verified model and screenshot files. Inspect Cubism before retrying.`)
  }
}

// Dependencies and fixed recipe selection are internal/test options, never tool input.
export function createEyeRigRunner({ root = packageRoot, platform = process.platform, systemRoot = process.env.SystemRoot, executeFile = runFile, timeoutMs = EYE_RIG_TIMEOUT_MS, recipe = 'eye' } = {}) {
  if (!Object.hasOwn(recipes, recipe)) throw new Error('Unknown fixed Cubism rig recipe.')
  const { script: recipeScript, output, label, name } = recipes[recipe]
  return async function buildRig(args, callerSignal) {
    if (!args || typeof args !== 'object' || Array.isArray(args) || Object.keys(args).length) throw new Error(`The ${name} tool takes no arguments.`)
    if (platform !== 'win32' || !systemRoot || !/^[A-Za-z]:[\\\\/]/.test(systemRoot)) throw new Error(`The ${name} recipe requires Windows PowerShell and a local Cubism Editor desktop.`)

    const timeout = AbortSignal.timeout(timeoutMs)
    const signal = callerSignal ? AbortSignal.any([callerSignal, timeout]) : timeout
    function checkCancelled() {
      if (callerSignal?.aborted) throw new Error(`${label} cancelled. Inspect Cubism before retrying; desktop edits may remain.`)
      if (timeout.aborted) throw new Error(`${label} timed out. Inspect Cubism before retrying; desktop edits may remain.`)
    }
    checkCancelled()

    let cwd, script, directory, lockPath
    try {
      cwd = await realpath(root)
      const workspace = resolve(cwd, '../..')
      script = await verifiedFile(cwd, resolve(cwd, 'scripts', recipeScript), '.ps1')
      // Walk the fixed path so existing redirects are rejected before deeper writes.
      directory = workspace
      for (const part of ['.local', 'qa', output]) {
        directory = resolve(directory, part)
        try { await mkdir(directory) } catch (error) { if (error.code !== 'EEXIST') throw error }
        if (!(await lstat(directory)).isDirectory() || relative(directory, await realpath(directory)) !== '') throw new Error()
        // Both recipes control the same desktop; lock the shared verified QA parent.
        if (part === 'qa') lockPath = resolve(directory, '.cubism-rig.lock')
      }
    } catch {
      checkCancelled()
      throw new Error(`${label} recipe or its local QA output directory is unavailable.`)
    }

    checkCancelled()
    let lock
    try {
      lock = await open(lockPath, 'wx')
    } catch (error) {
      if (error.code === 'EEXIST') throw new Error('A Cubism rig run already holds the QA lock. Do not retry until it has stopped.')
      throw new Error(`${label} could not acquire the local QA lock.`)
    }

    try {
      checkCancelled()
      const powershell = win32.resolve(systemRoot, 'System32/WindowsPowerShell/v1.0/powershell.exe')
      let stdout
      try {
        const result = await executeFile(powershell, [
          '-NoLogo', '-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', script, '-OutputDirectory', directory,
        ], { cwd, signal, windowsHide: true, shell: false, encoding: 'utf8', maxBuffer: 64 * 1024 })
        stdout = result.stdout
      } catch {
        checkCancelled()
        throw new Error(`${label} automation failed. Check that the imported fixture is active in Cubism FREE and inspect the Editor before retrying.`)
      }
      checkCancelled()
      const result = await verifyEyeRigOutput(directory, stdout, label)
      checkCancelled()
      return result
    } finally {
      try {
        await lock.close()
        await unlink(lockPath)
      } catch {
        throw new Error(`${label} could not release the local QA lock. Inspect the Editor and output before retrying.`)
      }
    }
  }
}

export const buildEyeRig = createEyeRigRunner()
export const buildFaceRig = createEyeRigRunner({ recipe: 'face' })
