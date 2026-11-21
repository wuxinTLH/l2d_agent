import type { Asset } from './domain'
import type { PsdDraft } from './library-model'
export const cubismParts = [
 {name:'草图',order:500},{name:'脸',order:500},{name:'眼睛',order:600},{name:'眼球',order:600},{name:'眉毛',order:600},{name:'嘴',order:600},{name:'鼻子',order:600},{name:'耳朵',order:500},{name:'前发',order:700},{name:'侧发',order:500},{name:'后发',order:300},{name:'脖子',order:400},{name:'身体',order:400},{name:'背景',order:0},{name:'[参考图]',order:1000},
] as const
export function cubismPart(name:string,reference=false):string {
 name=name.replace(/[（(][^）)]*[）)]/g,'').trim()
 if(reference) return '[参考图]'
 if(/参考|原图|^generated[\s_-]+oc/i.test(name))return '[参考图]'
 if(/草图|sketch/i.test(name))return '草图'
 if(/眼球|瞳|虹膜|iris|pupil/i.test(name))return '眼球'
 if(/眉|brow/i.test(name))return '眉毛'
 if(/眼|eye/i.test(name))return '眼睛'
 if(/嘴|mouth/i.test(name))return '嘴'
 if(/鼻|nose/i.test(name))return '鼻子'
 if(/耳|ear/i.test(name))return '耳朵'
 if(/后发|後髮|back.?hair/i.test(name))return '后发'
 if(/侧发|側髮|side.?hair/i.test(name))return '侧发'
 if(/前发|刘海|front.?hair|bang/i.test(name))return '前发'
 if(/头发|hair/i.test(name))return '侧发'
 if(/脖|颈|neck/i.test(name))return '脖子'
 if(/头|脸|face|head/i.test(name))return '脸'
 if(/背景|background/i.test(name))return '背景'
 if(/身|躯|手|腿|脚|body|torso|hand|arm|leg|foot/i.test(name))return '身体'
 return '未分类'
}
export function applyCubismTemplate(draft:PsdDraft,assets:Asset[]):PsdDraft {
 const sources=new Set(assets.map(a=>a.sourcePlacement?.assetId).filter(Boolean))
 const layers=draft.layers.map((layer,index)=>{const asset=assets.find(a=>a.id===layer.assetId);const group=cubismPart(layer.name,sources.has(layer.assetId));return {layer:{...layer,group},index,order:cubismParts.find(p=>p.name===group)?.order??400}})
 layers.sort((a,b)=>b.order-a.order||cubismParts.findIndex(p=>p.name===a.layer.group)-cubismParts.findIndex(p=>p.name===b.layer.group)||a.index-b.index)
 // References are retained at the bottom instead of obscuring editable parts.
 layers.sort((a,b)=>Number(a.layer.group==='[参考图]')-Number(b.layer.group==='[参考图]'))
 return {...draft,layers:layers.map(v=>v.layer)}
}
export function restoreSourcePositions(draft:PsdDraft,assets:Asset[]):{draft:PsdDraft;restored:number;unverified:number}{
 let restored=0,unverified=0
 const layers=draft.layers.map(layer=>{const asset=assets.find(a=>a.id===layer.assetId);const p=asset?.sourcePlacement;if(!p||!p.verifiedScale||p.canvasWidth!==draft.width||p.canvasHeight!==draft.height){unverified++;return {...layer}}restored++;return {...layer,x:p.x,y:p.y}})
 return {draft:{...draft,layers},restored,unverified}
}
