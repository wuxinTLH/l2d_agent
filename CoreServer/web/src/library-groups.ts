// Group metadata is browser-local only. Asset files and server records are never modified.
export const LIBRARY_GROUPS_KEY = 'oc-live2d-library-groups-v1'
export interface LibraryGroup { id:string; name:string }
export interface LibraryGroups { version:1; groups:LibraryGroup[]; assignments:Record<string,string> }
export type GroupFilter = { kind:'all' } | { kind:'ungrouped' } | { kind:'group'; id:string }
export const emptyLibraryGroups = ():LibraryGroups => ({version:1,groups:[],assignments:{}})
const record = (value:unknown):value is Record<string,unknown> => value !== null && typeof value === 'object' && !Array.isArray(value)
const validId = (id:unknown):id is string => typeof id === 'string' && id.length > 0 && id.length <= 256
const validGroupId = (id:unknown):id is string => validId(id) && id !== 'all' && id !== 'ungrouped'
export function groupNameError(name:string, groups:LibraryGroup[], exceptId?:string):string | undefined {
  const trimmed = name.trim()
  if (!trimmed || trimmed.length > 60 || /[\u0000-\u001f\u007f]/.test(trimmed)) return '分组名称须为 1–60 个字符，不能包含控制字符。'
  if (groups.some(g=>g.id !== exceptId && g.name.toLowerCase() === trimmed.toLowerCase())) return '已存在同名分组。'
}
export function parseLibraryGroups(raw:string | null):LibraryGroups {
  if (raw === null) return emptyLibraryGroups()
  const value:unknown = JSON.parse(raw)
  if (!record(value) || value.version !== 1 || !Array.isArray(value.groups) || !record(value.assignments)) throw new Error('无效的分组存储格式。')
  const groups:LibraryGroup[] = []
  for (const group of value.groups) {
    if (!record(group) || !validGroupId(group.id) || typeof group.name !== 'string' || groups.some(g=>g.id === group.id) || groupNameError(group.name,groups) || group.name !== group.name.trim()) throw new Error('无效或重复的分组。')
    groups.push({id:group.id,name:group.name})
  }
  const ids = new Set(groups.map(g=>g.id))
  const assignments:[string,string][] = []
  for (const [assetId,groupId] of Object.entries(value.assignments)) {
    if (!validId(assetId) || typeof groupId !== 'string' || !ids.has(groupId)) throw new Error('无效的素材分组映射。')
    assignments.push([assetId,groupId])
  }
  return {version:1,groups,assignments:Object.fromEntries(assignments)}
}
export function addLibraryGroup(state:LibraryGroups, id:string, name:string):LibraryGroups {
  const error = groupNameError(name,state.groups)
  if (error) throw new Error(error)
  if (!validGroupId(id) || state.groups.some(g=>g.id === id)) throw new Error('分组标识无效或重复。')
  return {...state,groups:[...state.groups,{id,name:name.trim()}]}
}
export function removeLibraryGroup(state:LibraryGroups, id:string):LibraryGroups {
  return {...state,groups:state.groups.filter(g=>g.id !== id),assignments:Object.fromEntries(Object.entries(state.assignments).filter(([,group])=>group !== id))}
}
export function assignLibraryGroup(state:LibraryGroups, assetIds:Iterable<string>, groupId:string | null):LibraryGroups {
  if (groupId !== null && !state.groups.some(g=>g.id === groupId)) throw new Error('分组已不存在。')
  const assignments = new Map(Object.entries(state.assignments))
  for (const id of assetIds) {
    if (!validId(id)) throw new Error('素材标识无效。')
    if (groupId === null) assignments.delete(id)
    else assignments.set(id,groupId)
  }
  return {...state,assignments:Object.fromEntries(assignments)}
}
export function assetGroup(state:LibraryGroups, id:string):string | undefined {
  return Object.prototype.hasOwnProperty.call(state.assignments,id) ? state.assignments[id] : undefined
}
export function filterLibraryAssets<T extends {id:string}>(assets:T[], state:LibraryGroups, filter:GroupFilter):T[] {
  return assets.filter(a=>filter.kind === 'all' || (filter.kind === 'ungrouped' ? !assetGroup(state,a.id) : assetGroup(state,a.id) === filter.id))
}
export function visibleSelection(checked:ReadonlySet<string>, visibleIds:Iterable<string>) {
  const ids = [...new Set(visibleIds)], selected = ids.filter(id=>checked.has(id)).length
  return {selected,total:ids.length,all:ids.length > 0 && selected === ids.length,hidden:checked.size-selected}
}
export function changeVisibleSelection(checked:ReadonlySet<string>, visibleIds:Iterable<string>, action:'toggle' | 'invert'):Set<string> {
  const ids = [...new Set(visibleIds)], next = new Set(checked)
  const deselect = action === 'toggle' && visibleSelection(checked,ids).all
  for (const id of ids) {
    if (deselect || (action === 'invert' && checked.has(id))) next.delete(id)
    else next.add(id)
  }
  return next
}
