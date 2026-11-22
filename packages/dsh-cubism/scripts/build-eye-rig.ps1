param([Parameter(Mandatory=$true)][string]$OutputDirectory)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices; using System.Text;
public class RigUI {
 public struct R {public int L,T,W,B;}
 public struct P {public int X,Y;}
 public delegate bool EnumProc(IntPtr h,IntPtr l);
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb,IntPtr l);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h,out R r);
 [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(P p);
 [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint x,uint y,uint d,UIntPtr e);
}
'@
[RigUI]::SetProcessDPIAware() | Out-Null
$fixture='8b2b9c88e31a0519f6fe2cf3'
$workspace=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../..'))
$exports=Join-Path $workspace '.local/assets/exports'
$expectedOut=Join-Path $workspace '.local/qa/one-click-rig'
if ([IO.Path]::GetFullPath($OutputDirectory) -ne $expectedOut) {throw 'Unexpected output directory'}
$front=[RigUI]::GetForegroundWindow()
function Title($hw){$s=New-Object Text.StringBuilder 1024;[RigUI]::GetWindowText($hw,$s,1024)|Out-Null;$s.ToString()}
function Owner($hw){$id=0;[RigUI]::GetWindowThreadProcessId($hw,[ref]$id)|Out-Null;$id}
$editorId=Owner $front
$cmd=(Get-CimInstance Win32_Process -Filter "ProcessId=$editorId").CommandLine
if ((Title $front) -notmatch ('^Live2D Cubism Editor 5\.3\.05\s+\[ FREE[^\]]*\]\s+- '+$fixture+'$') -or $cmd -notmatch 'CECubismEditorApp|Live2D_Cubism\.jar') {throw 'Fresh fixture must be foreground in Cubism 5.3.05 FREE'}
$main=$front
function Rect($hw){$r=New-Object RigUI+R;if(![RigUI]::GetWindowRect($hw,[ref]$r)){throw 'Cannot read window geometry'};$r}
$r=Rect $main
if (($r.W-$r.L) -ne 2576 -or ($r.B-$r.T) -ne 1416) {throw 'Unsupported fixture window layout'}
$meshTitle=-join ([char[]]@(33258,21160,32593,26684,29983,25104))
function OtherWindows {
 $script:owned=@()
 [RigUI]::EnumWindows({param($hw,$unused) if($hw -ne $main -and [RigUI]::IsWindowVisible($hw) -and (Owner $hw) -eq $editorId -and (Title $hw)){$script:owned+= $hw};return $true},[IntPtr]::Zero)|Out-Null
 $script:owned
}
function Guard($hw){if([RigUI]::GetForegroundWindow() -ne $hw -or (Owner $hw) -ne $editorId){throw 'Cubism focus changed; stopped'};if($hw -eq $main -and @(OtherWindows).Count){throw 'Unexpected Cubism dialog; stopped'}}
function Point($hw,$x,$y){$b=Rect $hw;$p=New-Object RigUI+P;$p.X=$b.L+$x;$p.Y=$b.T+$y;if((Owner ([RigUI]::WindowFromPoint($p))) -ne $editorId){throw 'Target point is obscured'};$p}
function Click($hw,$x,$y){Guard $hw;$p=Point $hw $x $y;if(![RigUI]::SetCursorPos($p.X,$p.Y)){throw 'Cannot move cursor'};Guard $hw;[RigUI]::mouse_event(2,0,0,0,[UIntPtr]::Zero);[RigUI]::mouse_event(4,0,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 380}
function Keys($hw,$keys){Guard $hw;[Windows.Forms.SendKeys]::SendWait($keys);Start-Sleep -Milliseconds 550}
function Capture($hw){Guard $hw;$b=Rect $hw;$image=New-Object Drawing.Bitmap ($b.W-$b.L),($b.B-$b.T);$gfx=[Drawing.Graphics]::FromImage($image);try{$gfx.CopyFromScreen($b.L,$b.T,0,0,$image.Size);Guard $hw;return $image}catch{$image.Dispose();throw}finally{$gfx.Dispose()}}
function SaveShot($name){$bmp=Capture $main;try{$path=Join-Path $run ($name+'.png');$bmp.Save($path,[Drawing.Imaging.ImageFormat]::Png);$path}finally{$bmp.Dispose()}}
function PurpleCount($bmp){$count=0;for($y=490;$y -lt 900;$y+=3){for($x=1500;$x -lt 1950;$x+=3){$c=$bmp.GetPixel($x,$y);if(($c.B-$c.G) -gt 22 -and ($c.R-$c.G) -gt 7){$count++}}};$count}
Guard $main
$pre=Capture $main;try{if((PurpleCount $pre) -lt 100){throw 'Fixture eye is not at the expected zoom and location'}}finally{$pre.Dispose()}
$label='eye-rig-'+[Guid]::NewGuid().ToString('N').Substring(0,12)
$run=Join-Path $expectedOut $label
[IO.Directory]::CreateDirectory($run)|Out-Null
Click $main 1730 715
Click $main 1333 75
$dialog=[RigUI]::GetForegroundWindow()
if((Title $dialog) -ne $meshTitle -or (Owner $dialog) -ne $editorId){throw 'Automatic mesh dialog did not appear'}
$dr=Rect $dialog;if(($dr.W-$dr.L) -ne 295 -or ($dr.B-$dr.T) -ne 326){throw 'Unexpected mesh dialog geometry'}
Click $dialog 260 70
Click $dialog 49 135
Click $dialog 265 14
$mesh=SaveShot '01-auto-mesh'
Click $main 584 1110
Click $main 561 1005
Click $main 649 1110
# The fixture's selected mesh bounds differ from the earlier manually zoomed example.
# Find the red transform rectangle in its calibrated canvas region before dragging.
$bmp=Capture $main
try {
 $bx=0;$by=0;$best=0
 for($y=480;$y -lt 660;$y++){ $n=0;$sum=0
  for($x=1460;$x -lt 1940;$x++){ $c=$bmp.GetPixel($x,$y);if($c.R -gt 160 -and ($c.R-$c.G) -gt 55 -and ($c.R-$c.B) -gt 42){$n++;$sum+=$x}}
  if($n -gt $best){$best=$n;$bx=[int]($sum/$n);$by=$y}
 }
 if($best -lt 90){throw 'Selected mesh transform frame not found'}
} finally {$bmp.Dispose()}
Guard $main;$from=Point $main $bx $by;$target=Point $main $bx ($by+185)
[RigUI]::SetCursorPos($from.X,$from.Y)|Out-Null;Guard $main
[RigUI]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
try{for($i=1;$i -le 12;$i++){Guard $main;[RigUI]::SetCursorPos($from.X,$from.Y+[int](($target.Y-$from.Y)*$i/12))|Out-Null;Start-Sleep -Milliseconds 35}}finally{[RigUI]::mouse_event(4,0,0,0,[UIntPtr]::Zero)}
Start-Sleep -Milliseconds 500
$closed=SaveShot '02-closed'
Click $main 805 1110
$mid=SaveShot '03-half'
Click $main 961 1110
$open=SaveShot '04-open'
$pics=@($closed,$mid,$open);$counts=@(foreach($pic in $pics){$im=New-Object Drawing.Bitmap $pic;try{PurpleCount $im}finally{$im.Dispose()}})
if(!($counts[0]+15 -lt $counts[1] -and $counts[1]+15 -lt $counts[2])){throw "Eye did not visibly interpolate ($counts)"}
Keys $main '^s'
$saveTitle=-join ([char[]]@(20445,23384))
$save=[IntPtr]::Zero
for($i=0;$i -lt 30;$i++){
 $active=[RigUI]::GetForegroundWindow()
 if((Owner $active) -ne $editorId){throw 'Cubism focus changed while waiting for Save'}
 if($active -ne $main -and (Title $active) -in @($saveTitle,'Save')){$save=$active;break}
 Start-Sleep -Milliseconds 200
}
if($save -eq [IntPtr]::Zero){throw 'Expected a new-document Save dialog'}
$sr=Rect $save
if(($sr.W-$sr.L) -ne 816 -or ($sr.B-$sr.T) -ne 638){throw 'Unexpected Save dialog geometry'}
# Type only a generated safe basename, and never approve an overwrite prompt.
Click $save 350 540
Keys $save ('^a'+$label+'.cmo3')
Click $save 683 607
Start-Sleep -Milliseconds 900
$model=Join-Path $exports ($label+'.cmo3')
if(!(Test-Path -LiteralPath $model -PathType Leaf) -or (Get-Item -LiteralPath $model).Length -lt 1024 -or (Title $main) -notlike ('*'+$label+'.cmo3*')){throw 'New model save could not be verified'}
$dest=Join-Path $run ($label+'.cmo3')
[IO.File]::Copy($model,$dest,$false)
@{status='saved';modelPath=$dest;screenshots=@($mesh,$closed,$mid,$open)} | ConvertTo-Json -Compress
