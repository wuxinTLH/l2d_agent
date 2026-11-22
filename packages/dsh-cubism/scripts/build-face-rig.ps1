param([Parameter(Mandatory=$true)][string]$OutputDirectory)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type @'
using System;using System.Text;using System.Runtime.InteropServices;
public class FaceUI {
 public struct R{public int L,T,Rt,B;}
 [DllImport("user32.dll")]public static extern bool SetProcessDPIAware();
 [DllImport("user32.dll")]public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")]public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)]public static extern int GetWindowText(IntPtr h,StringBuilder b,int n);
 [DllImport("user32.dll")]public static extern bool GetWindowRect(IntPtr h,out R r);
 [DllImport("user32.dll")]public static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,uint d,UIntPtr e);
}
'@
[FaceUI]::SetProcessDPIAware()|Out-Null
$workspace=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../..'))
if([IO.Path]::GetFullPath($OutputDirectory) -ne (Join-Path $workspace '.local/qa/face-rig')){throw 'Unexpected output directory'}
function Title($h){$b=New-Object Text.StringBuilder 1024;[FaceUI]::GetWindowText($h,$b,1024)|Out-Null;$b.ToString()}
function Owner($h){$n=0;[FaceUI]::GetWindowThreadProcessId($h,[ref]$n)|Out-Null;$n}
$main=[FaceUI]::GetForegroundWindow();$editorId=Owner $main
if((Title $main) -notmatch '^Live2D Cubism Editor 5\.3\.05\s+\[ FREE[^\]]*\]\s+- face-fixture$'){throw 'Fresh face-fixture must be active in Cubism FREE'}
if((Get-CimInstance Win32_Process -Filter "ProcessId=$editorId").CommandLine -notmatch 'CECubismEditorApp|Live2D_Cubism.jar'){throw 'Wrong process'}
function Rect($h){$r=New-Object FaceUI+R;[FaceUI]::GetWindowRect($h,[ref]$r)|Out-Null;$r}
$r=Rect $main;if(($r.Rt-$r.L)-ne 2576 -or ($r.B-$r.T)-ne 1416){throw 'Unsupported layout'}
function Guard($h){if([FaceUI]::GetForegroundWindow()-ne $h -or (Owner $h)-ne $editorId){throw 'Focus changed; stopped'}}
function Click($h,$x,$y){Guard $h;$r=Rect $h;[FaceUI]::SetCursorPos($r.L+$x,$r.T+$y)|Out-Null;Guard $h;[FaceUI]::mouse_event(2,0,0,0,[UIntPtr]::Zero);[FaceUI]::mouse_event(4,0,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 260}
function Keys($h,$s){Guard $h;[Windows.Forms.SendKeys]::SendWait($s);Start-Sleep -Milliseconds 400}
function Drag($x,$y,$dy){Guard $main;$r=Rect $main;[FaceUI]::SetCursorPos($r.L+$x,$r.T+$y)|Out-Null;[FaceUI]::mouse_event(2,0,0,0,[UIntPtr]::Zero);try{for($i=1;$i-le 10;$i++){Guard $main;[FaceUI]::SetCursorPos($r.L+$x,$r.T+$y+[int]($dy*$i/10))|Out-Null;Start-Sleep -Milliseconds 30}}finally{[FaceUI]::mouse_event(4,0,0,0,[UIntPtr]::Zero)};Start-Sleep -Milliseconds 300}
$label='face-rig-'+[Guid]::NewGuid().ToString('N').Substring(0,12);$run=Join-Path $OutputDirectory $label;[IO.Directory]::CreateDirectory($run)|Out-Null
function Shot($name){Guard $main;$r=Rect $main;$b=New-Object Drawing.Bitmap 2576,1416;$g=[Drawing.Graphics]::FromImage($b);try{$g.CopyFromScreen($r.L,$r.T,0,0,$b.Size);Guard $main;$p=Join-Path $run ($name+'.png');$b.Save($p);$p}finally{$g.Dispose();$b.Dispose()}}
function Mesh($row){Click $main 145 $row;Click $main 1333 75;$d=[FaceUI]::GetForegroundWindow();if((Owner $d)-ne $editorId -or $d-eq $main){throw 'Mesh dialog missing'};$q=Rect $d;if(($q.Rt-$q.L)-ne 295 -or ($q.B-$q.T)-ne 326){throw 'Wrong mesh dialog'};Click $d 260 70;Click $d 49 135;Click $d 265 14}
# Fresh fixture has sixteen independently drawn layers. Only eyelid skin covers
# translate; iris and sclera geometry are never scaled or moved.
Mesh 342
Click $main 584 1110;Click $main 561 1005;Click $main 649 1110
# Anatomical left eye is viewer-right. Translate the lid only.
Drag 2034 565 130
$left=Shot '01-left-closed'
Click $main 961 1110
Mesh 254
Click $main 584 1162;Click $main 561 1005;Click $main 649 1162
Drag 1587 565 130
$right=Shot '02-right-closed'
Click $main 649 1110
$closed=Shot '03-both-closed'
Click $main 805 1110;Click $main 805 1162
$half=Shot '04-both-half'
Click $main 961 1110;Click $main 961 1162
$open=Shot '05-both-open'
# Mouth: separate cavity/tongue stay unchanged; upper lip/skin cover translates.
Mesh 430
Guard $main;$r=Rect $main;[FaceUI]::SetCursorPos($r.L+703,$r.T+1210)|Out-Null
[FaceUI]::mouse_event(2048,0,0,[BitConverter]::ToUInt32([BitConverter]::GetBytes([int]-600),0),[UIntPtr]::Zero);Start-Sleep -Milliseconds 450
Click $main 584 1138;Click $main 961 1138;Click $main 561 1005;Click $main 649 1138
Drag 1810 846 116
$mouthClosed=Shot '06-mouth-closed'
Click $main 805 1138;$mouthHalf=Shot '07-mouth-half'
Click $main 961 1138
# Give the independently drawn tongue its own mouth-linked motion. The tongue
# recedes downward on closure; neither its pixels nor the whole mouth are scaled.
Mesh 474
Click $main 584 1138;Click $main 961 1138;Click $main 561 1005;Click $main 649 1138
Drag 1810 987 28
Click $main 961 1138;$mouthOpen=Shot '08-mouth-open-tongue'
# Reject a run that merely moved the wrong object or changed the other eye.
function BluePixels($path,$x0){$b=New-Object Drawing.Bitmap $path;try{$n=0;for($y=620;$y-lt 770;$y+=2){for($x=$x0;$x-lt ($x0+160);$x+=2){$c=$b.GetPixel($x,$y);if($c.B-gt ($c.R*1.15) -and $c.G-gt ($c.R*1.1) -and $c.B-gt 60){$n++}}};$n}finally{$b.Dispose()}}
if((BluePixels $left 1960)-gt 30 -or (BluePixels $left 1510)-lt 1000 -or (BluePixels $right 1510)-gt 30 -or (BluePixels $right 1960)-lt 1000){throw 'Independent eyelid visual verification failed'}
Keys $main '^s'
$save=[IntPtr]::Zero
for($i=0;$i-lt 35;$i++){$h=[FaceUI]::GetForegroundWindow();if((Owner $h)-ne $editorId){throw 'Focus changed while saving'};if($h-ne $main -and (Title $h)-in @('Save',(-join([char[]]@(20445,23384))))){$save=$h;break};Start-Sleep -Milliseconds 180}
if($save-eq [IntPtr]::Zero){throw 'Save dialog missing'}
$sr=Rect $save;if(($sr.Rt-$sr.L)-ne 816 -or ($sr.B-$sr.T)-ne 638){throw 'Unexpected save layout'}
Click $save 350 540;Keys $save ('^a'+$label+'.cmo3');Click $save 683 607;Start-Sleep -Milliseconds 1000
# Java file chooser starts beside imported PSD. Never overwrite existing files.
$model=Join-Path $workspace ('.local/qa/face-fixture/'+$label+'.cmo3')
if(!(Test-Path -LiteralPath $model)){$model=Join-Path $workspace ('.local/assets/exports/'+$label+'.cmo3')}
if(!(Test-Path -LiteralPath $model) -or (Get-Item -LiteralPath $model).Length -lt 1024){throw 'Saved model not found in isolated export locations'}
$dest=Join-Path $run ($label+'.cmo3');[IO.File]::Copy($model,$dest,$false)
@{status='saved';modelPath=$dest;screenshots=@($left,$right,$closed,$half,$open,$mouthClosed,$mouthHalf,$mouthOpen)}|ConvertTo-Json -Compress
