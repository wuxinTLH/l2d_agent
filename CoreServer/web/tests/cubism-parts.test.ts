import {test} from 'node:test'
import assert from 'node:assert/strict'
import {cubismPart,applyCubismTemplate,restoreSourcePositions} from '../src/cubism-parts.ts'
import type {Asset} from '../src/domain.ts'
const asset=(id:string,name:string):Asset=>({id,name,url:'/assets/a.png',width:20,height:30,createdAt:''})
test('Cubism screenshot categories and stack preserve source provenance',()=>{
 assert.equal(cubismPart('头（不含眼睛及嘴及耳朵及头发）'),'脸');assert.equal(cubismPart('身体（不含头及手及头发）'),'身体');assert.equal(cubismPart('左眼球'),'眼球');assert.equal(cubismPart('右眼睛'),'眼睛');assert.equal(cubismPart('后发'),'后发');assert.equal(cubismPart('鼻子'),'鼻子')
 const a=asset('a','原始角色'),b=asset('b','前发'),c=asset('c','眼球');b.sourcePlacement={assetId:'a',x:2,y:3,canvasWidth:100,canvasHeight:200,verifiedScale:true}
 const d={width:100,height:200,layers:[a,c,b].map(v=>({assetId:v.id,name:v.name,x:0,y:0}))};const out=applyCubismTemplate(d,[a,b,c]);assert.deepEqual(out.layers.map(l=>l.group),['前发','眼球','[参考图]']);assert.equal(d.layers[0].assetId,'a')
})
test('automatic source positions require verified matching canvas; preserve unknown/manual values',()=>{const a=asset('a','眼睛');a.sourcePlacement={assetId:'s',x:12,y:23,canvasWidth:100,canvasHeight:200,verifiedScale:true};const d={width:100,height:200,layers:[{assetId:'a',name:'眼睛',x:5,y:6}]};let r=restoreSourcePositions(d,[a]);assert.equal(r.restored,1);assert.equal(r.draft.layers[0].x,12);assert.equal(d.layers[0].x,5);a.sourcePlacement.verifiedScale=false;r=restoreSourcePositions(d,[a]);assert.equal(r.unverified,1);assert.equal(r.draft.layers[0].x,5)})
