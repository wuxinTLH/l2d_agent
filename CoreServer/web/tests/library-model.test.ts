import test from 'node:test'
import assert from 'node:assert/strict'
import { createPsdDraft, deleteSequential, psdDownloadUrl, psdDraftWarnings, validatePsdDraft } from '../src/library-model.ts'
const assets = [{id:'a',name:'<body>',url:'/assets/a.png',width:100,height:200,createdAt:''},{id:'b',name:'hair',url:'/assets/b.png',width:300,height:80,createdAt:''}]
test('PSD defaults place hair above body, use maximum dimensions and never scale',()=> {
  assert.deepEqual(createPsdDraft(assets),{width:300,height:200,layers:[{assetId:'b',name:'hair',x:0,y:0},{assetId:'a',name:'<body>',x:0,y:0}]})
})
test('PSD defaults stably rank facial parts, hair, head/body, other parts and originals',()=> {
  const names=['Generated OC','body','back hair','left_eye','mouth','eyebrows','head','front hair','right-eye','accessory','Generated OC (2)']
  const selection=names.map((name,index)=>({...assets[0],id:String(index),name}))
  const before=structuredClone(selection)
  assert.deepEqual(createPsdDraft(selection).layers.map(l=>l.name),['left_eye','mouth','eyebrows','right-eye','back hair','front hair','body','head','accessory','Generated OC','Generated OC (2)'])
  assert.deepEqual(selection,before)
})
test('source asset IDs identify renamed originals and derived names are not mistaken for originals',()=> {
  const original={...assets[0],id:'source',name:'mouth'}
  const part={...assets[0],id:'part',name:'Generated OC',sourcePlacement:{assetId:'source',x:12,y:34,canvasWidth:100,canvasHeight:200,verifiedScale:true}}
  assert.deepEqual(createPsdDraft([original,part]).layers,[{assetId:'part',name:'Generated OC',x:12,y:34},{assetId:'source',name:'mouth',x:0,y:0}])
})
test('Chinese facial and anatomical names use the same stable defaults',()=> {
  const selection=['身体','头发','眉毛','嘴巴','左眼','头部'].map((name,index)=>({...assets[0],id:String(index),name}))
  assert.deepEqual(createPsdDraft(selection).layers.map(l=>l.name),['眉毛','嘴巴','左眼','头发','身体','头部'])
})
test('validation and warnings never undo a manual layer reorder or offsets',()=> {
  const draft=createPsdDraft(assets)
  draft.layers.reverse(); draft.layers[0].x=17; draft.layers[1].y=-5
  const before=structuredClone(draft)
  assert.equal(validatePsdDraft(draft,assets),undefined)
  assert.deepEqual(psdDraftWarnings(draft,assets),[])
  assert.deepEqual(draft,before)
})
test('unverified placement warns without blocking exports or warning on legacy assets',()=> {
  const unverified={...assets[0],sourcePlacement:{x:0,y:0,canvasWidth:100,canvasHeight:200,verifiedScale:false}}
  const verified={...assets[1],sourcePlacement:{x:0,y:0,canvasWidth:100,canvasHeight:200,verifiedScale:true}}
  const selection=[unverified,verified]
  const draft=createPsdDraft(selection)
  assert.equal(validatePsdDraft(draft,selection),undefined)
  const warnings=psdDraftWarnings(draft,selection)
  assert.equal(warnings.length,1)
  assert.match(warnings[0],/尚未验证原图比例与位置/)
  assert.deepEqual(psdDraftWarnings({...draft,layers:draft.layers.filter(l=>l.assetId!=='a')},selection),[])
  assert.deepEqual(psdDraftWarnings(createPsdDraft(assets),assets),[])
})
test('compact parts restore source canvas and offsets without resizing',()=> {
  const part={...assets[0],width:30,height:12,sourcePlacement:{x:45,y:60,canvasWidth:1024,canvasHeight:1536,verifiedScale:true}}
  assert.deepEqual(createPsdDraft([part]),{width:1024,height:1536,layers:[{assetId:'a',name:'<body>',x:45,y:60}]})
})
test('PSD validation rejects empty, oversized, missing and invalid fields',()=> {
  const draft=createPsdDraft(assets)
  assert.equal(validatePsdDraft(draft,assets),undefined)
  assert.ok(validatePsdDraft({...draft,layers:[]},assets))
  assert.ok(validatePsdDraft({...draft,layers:Array(65).fill(draft.layers[0])},assets))
  assert.ok(validatePsdDraft({...draft,width:NaN},assets))
  assert.ok(validatePsdDraft({...draft,height:0},assets))
  assert.ok(validatePsdDraft({...draft,width:8193},assets))
  assert.ok(validatePsdDraft({...draft,width:8192,height:8192},assets))
  assert.equal(validatePsdDraft({...draft,layers:Array(64).fill(draft.layers[0])},assets),undefined)
  assert.ok(validatePsdDraft({...draft,layers:[{...draft.layers[0],x:8193}]},assets))
  assert.ok(validatePsdDraft(draft,[]))
  for(const layer of [{...draft.layers[0],name:' '},{...draft.layers[0],x:0.5},{...draft.layers[0],y:Infinity}]) assert.ok(validatePsdDraft({...draft,layers:[layer]},assets))
  assert.equal(validatePsdDraft({...draft,layers:[{...draft.layers[0],x:-12,y:220}]},assets),undefined)
})
test('download URL accepts only matching server-local PSD route',()=> {
  const result={id:'psd_test-1',url:'/exports/psd_test-1.psd',width:10,height:20,layerCount:2}
  assert.equal(psdDownloadUrl(result),result.url)
  for (const url of ['javascript:alert(1)','https://evil.test/x.psd','//evil.test/x.psd','/exports/other.psd','/exports/psd_test-1.psd?x=1']) assert.throws(()=>psdDownloadUrl({...result,url}))
  assert.throws(()=>psdDownloadUrl({...result,id:'../bad'}))
})
test('deletions run sequentially, preserve failed items and continue after failures',async()=> {
  const calls:string[]=[];let active=0
  const result=await deleteSequential(['a','b','c'],async id=> {assert.equal(active,0);active++;calls.push(id);await Promise.resolve();active--;if(id==='b')throw new Error('locked')})
  assert.deepEqual(calls,['a','b','c'])
  assert.deepEqual(result,{deleted:['a','c'],failed:[{id:'b',message:'locked'}]})
})
