import test from 'node:test'
import assert from 'node:assert/strict'
import { addLibraryGroup, assignLibraryGroup, assetGroup, changeVisibleSelection, emptyLibraryGroups, filterLibraryAssets, groupNameError, parseLibraryGroups, removeLibraryGroup, visibleSelection } from '../src/library-groups.ts'

const grouped = () => addLibraryGroup(addLibraryGroup(emptyLibraryGroups(),'g1',' Face '),'g2','Hair')
test('group names trim, reject empty, long, controls and case-insensitive duplicates',()=> {
  const state = grouped()
  assert.equal(state.groups[0].name,'Face')
  for (const name of ['', '  ', 'x'.repeat(61), 'a\nb', ' face ', 'HAIR']) assert.ok(groupNameError(name,state.groups))
  assert.equal(groupNameError('Face',state.groups,'g1'),undefined)
  assert.throws(()=>addLibraryGroup(state,'g3','FACE'))
  for (const id of ['g1','','all','ungrouped']) assert.throws(()=>addLibraryGroup(state,id,'Other'))
  assert.equal(addLibraryGroup(state,'g3','<img onerror="alert(1)">').groups[2].name,'<img onerror="alert(1)">')
})
test('assign, filter and delete group never mutate assets or prior state; missing assets retain mappings',()=> {
  const assets = [{id:'a'},{id:'b'},{id:'c'}], before = grouped()
  const state = assignLibraryGroup(before,['a','missing'],'g1')
  assert.deepEqual(before.assignments,{})
  assert.deepEqual(filterLibraryAssets(assets,state,{kind:'group',id:'g1'}),[{id:'a'}])
  assert.deepEqual(filterLibraryAssets(assets,state,{kind:'ungrouped'}),[{id:'b'},{id:'c'}])
  assert.deepEqual(filterLibraryAssets(assets,state,{kind:'all'}),assets)
  assert.deepEqual(filterLibraryAssets([],state,{kind:'all'}),[])
  assert.equal(state.assignments.missing,'g1')
  const moved = assignLibraryGroup(state,['a'],'g2')
  assert.equal(assetGroup(moved,'a'),'g2')
  assert.equal(assetGroup(assignLibraryGroup(moved,['a'],null),'a'),undefined)
  const deleted = removeLibraryGroup(state,'g1')
  assert.deepEqual(deleted.groups,[{id:'g2',name:'Hair'}])
  assert.deepEqual(deleted.assignments,{})
  assert.deepEqual(assets,[{id:'a'},{id:'b'},{id:'c'}])
  assert.equal(state.assignments.a,'g1')
  assert.throws(()=>assignLibraryGroup(state,['a'],'no-group'))
})
test('version 1 JSON round-trips arbitrary safe strings and special property names',()=> {
  const state = assignLibraryGroup(grouped(),['__proto__','constructor','<b>'],'g1')
  assert.deepEqual(parseLibraryGroups(JSON.stringify(state)),state)
  assert.equal(assetGroup(state,'__proto__'),'g1')
  assert.equal(assetGroup(emptyLibraryGroups(),'constructor'),undefined)
  assert.deepEqual(parseLibraryGroups(null),emptyLibraryGroups())
})
test('malformed or unsupported local storage fails explicitly, allowing caller to preserve original',()=> {
  for (const raw of ['', 'null','[]','{', '{}', '{"version":2,"groups":[],"assignments":{}}']) assert.throws(()=>parseLibraryGroups(raw))
  const valid = grouped()
  for (const value of [
    {...valid,groups:null}, {...valid,assignments:[]}, {...valid,groups:[{id:'g1',name:''}]},
    {...valid,groups:[{id:'g1',name:'A'},{id:'g1',name:'B'}]},
    {...valid,groups:[{id:'g1',name:'A'},{id:'g2',name:'a'}]},
    {...valid,groups:[{id:'all',name:'A'}]}, {...valid,assignments:{a:'missing'}},
    {...valid,assignments:{a:12}}, {...valid,groups:[{id:'g1',name:' A '}]},
  ]) assert.throws(()=>parseLibraryGroups(JSON.stringify(value)))
})
test('visible select-all toggles and inverse preserve hidden selection, handle partial/empty/duplicate lists',()=> {
  const initial = new Set(['hidden','a'])
  const all = changeVisibleSelection(initial,['a','b'],'toggle')
  assert.deepEqual([...all],['hidden','a','b'])
  assert.deepEqual(visibleSelection(all,['a','b']),{selected:2,total:2,all:true,hidden:1})
  assert.deepEqual([...changeVisibleSelection(all,['a','b'],'toggle')],['hidden'])
  assert.deepEqual([...changeVisibleSelection(initial,['a','b','b'],'invert')],['hidden','b'])
  assert.deepEqual([...changeVisibleSelection(all,['a','b'],'invert')],['hidden'])
  assert.deepEqual([...changeVisibleSelection(initial,[],'toggle')],[...initial])
  assert.deepEqual(visibleSelection(initial,[]),{selected:0,total:0,all:false,hidden:2})
  assert.deepEqual([...initial],['hidden','a'])
})
