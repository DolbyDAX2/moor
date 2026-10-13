#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef SourceDir
  #error SourceDir is required
#endif
#ifndef OutputPath
  #error OutputPath is required
#endif

[Setup]
AppId={{9B34383D-2615-4906-A267-DF09C45840C7}
AppName=Moor
AppVersion={#AppVersion}
AppPublisher=DolbyDAX2
AppPublisherURL=https://github.com/DolbyDAX2/moor
AppSupportURL=https://github.com/DolbyDAX2/moor/issues
AppUpdatesURL=https://github.com/DolbyDAX2/moor/releases
DefaultDirName={localappdata}\Programs\Moor
DefaultGroupName=Moor
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
OutputDir={#OutputPath}
OutputBaseFilename=moor-{#AppVersion}-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile={#SourceDir}\moor.ico
UninstallDisplayIcon={app}\moor.exe
CloseApplications=yes
VersionInfoVersion={#AppVersion}.0
VersionInfoProductName=Moor
VersionInfoDescription=Moor Setup

[Languages]
Name: "turkish"; MessagesFile: "compiler:Languages\Turkish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\moor.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\MicrosoftEdgeWebview2Setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall

[Icons]
Name: "{group}\Moor"; Filename: "{app}\moor.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Moor"; Filename: "{app}\moor.exe"; WorkingDir: "{app}"; Tasks: desktopicon
Name: "{group}\{cm:UninstallProgram,Moor}"; Filename: "{uninstallexe}"

[Run]
; Microsoft bootstrapper checks for an existing runtime and installs it if needed.
; A missing runtime requires an Internet connection. Installation is per-user.
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Microsoft Edge WebView2 Runtime..."; Flags: waituntilterminated
Filename: "{app}\moor.exe"; Description: "{cm:LaunchProgram,Moor}"; Flags: nowait postinstall skipifsilent

; User data in %APPDATA%\moor is intentionally not removed during uninstall.
