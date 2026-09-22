param([string]$Root, [string]$Source)
$ErrorActionPreference = "Stop"
Write-Output "Loading native drag peer assemblies"
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Write-Output "Compiling native drag peer"
Add-Type -ReferencedAssemblies System.Windows.Forms,System.Drawing -TypeDefinition @"
using System;
using System.IO;
using System.Drawing;
using System.Windows.Forms;
using System.Runtime.InteropServices;
public class WoxDragPeer : Form {
 [DllImport("user32.dll")] static extern bool SetProcessDpiAwarenessContext(IntPtr context);
 string root, source;
 protected override bool ShowWithoutActivation { get { return true; } }
 public WoxDragPeer(string root, string source) {
  this.root=root; this.source=source;
  Text="Wox smoke drag peer"; Size=new Size(300,220); TopMost=true;
  StartPosition=FormStartPosition.Manual;
  var area=Screen.PrimaryScreen.WorkingArea;
  Location=new Point(area.Right-Width-10, area.Bottom-Height-10);
  AllowDrop=true;
  MouseDown += (s,e) => {
   if(e.Button!=MouseButtons.Left) return;
   File.WriteAllText(Path.Combine(root,"started"),"ready");
   DoDragDrop(new DataObject(DataFormats.FileDrop,new string[]{source}),DragDropEffects.Copy);
   File.WriteAllText(Path.Combine(root,"ended"),"ready");
  };
  GiveFeedback += (s,e) => { if(e.Effect==DragDropEffects.Copy && !Bounds.Contains(Cursor.Position)) File.WriteAllText(Path.Combine(root,"accepted"),"copy"); };
  DragEnter += (s,e) => { e.Effect=e.Data.GetDataPresent(DataFormats.FileDrop)?DragDropEffects.Copy:DragDropEffects.None; File.WriteAllText(Path.Combine(root,"entered"),"ready"); };
  DragOver += (s,e) => { e.Effect=DragDropEffects.Copy; };
  DragDrop += (s,e) => {
   var files=(string[])e.Data.GetData(DataFormats.FileDrop);
   foreach(var file in files) File.Copy(file,Path.Combine(root,"received-"+Path.GetFileName(file)),true);
   File.WriteAllLines(Path.Combine(root,"received"),files);
   e.Effect=DragDropEffects.Copy;
  };
  Shown += (s,e) => { File.WriteAllText(Path.Combine(root,"ready"),Handle.ToInt64().ToString()); };
 }
 public static void Run(string root,string source) {
  SetProcessDpiAwarenessContext(new IntPtr(-4));
  Application.Run(new WoxDragPeer(root,source));
 }
}
"@
Write-Output "Starting native drag peer window"
[WoxDragPeer]::Run($Root, $Source)
