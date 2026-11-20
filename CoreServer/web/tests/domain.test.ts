import test from 'node:test'
import assert from 'node:assert/strict'
import { pixelRect, validateParts } from '../src/domain.ts'

test('forward drag uses integer pixel coordinates', () => {
  assert.deepEqual(pixelRect({x:10.3,y:20.2},{x:100.2,y:150.7},1024,1536),{x:10,y:20,width:90,height:131})
})
test('reverse drag produces the same rectangle', () => {
  assert.deepEqual(pixelRect({x:100,y:150},{x:10,y:20},1024,1536),{x:10,y:20,width:90,height:130})
})
test('drag coordinates are clamped to image bounds', () => {
  assert.deepEqual(pixelRect({x:-10,y:-5},{x:110,y:201},100,200),{x:0,y:0,width:100,height:200})
})
test('empty clicks keep a positive rectangle within image', () => {
  assert.deepEqual(pixelRect({x:100,y:200},{x:100,y:200},100,200),{x:99,y:199,width:1,height:1})
})
test('valid parts allow the exact image edge', () => {
  assert.equal(validateParts([{name:'body',x:0,y:0,width:100,height:200}],100,200),undefined)
})
test('rejects empty, duplicate, fractional, negative and out-of-range parts', () => {
  assert.ok(validateParts([],100,200))
  const part = {name:'body',x:0,y:0,width:100,height:200}
  assert.ok(validateParts([{...part,name:' '}],100,200))
  assert.ok(validateParts([part,{...part,name:' body '}],100,200))
  assert.ok(validateParts([{...part,x:0.5}],100,200))
  assert.ok(validateParts([{...part,x:-1}],100,200))
  assert.ok(validateParts([{...part,width:0}],100,200))
  assert.ok(validateParts([{...part,width:101}],100,200))
  assert.ok(validateParts([{...part,x:NaN}],100,200))
})
