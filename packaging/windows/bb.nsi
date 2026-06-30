; NSIS installer for bb (Bitbucket CLI), built with makensis on Linux CI.
; Build via packaging/windows/build-nsis.sh, which passes:
;   -DVERSION=<version> -DOUTFILE=<path> -DSRCDIR=<dir containing bb.exe>
; PATH handling uses the EnVar plugin (https://nsis.sourceforge.io/EnVar_plug-in).

Unicode true

!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef OUTFILE
  !define OUTFILE "bb-setup.exe"
!endif
!ifndef SRCDIR
  !define SRCDIR "."
!endif

!define APPNAME "bb (Bitbucket CLI)"
!define PUBLISHER "delabrcd"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\bb"

Name "${APPNAME}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\bb"
InstallDirRegKey HKLM "Software\bb" "InstallDir"
RequestExecutionLevel admin
ShowInstDetails show
ShowUninstDetails show

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "bb (required)" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "${SRCDIR}\bb.exe"

  WriteRegStr HKLM "Software\bb" "InstallDir" "$INSTDIR"

  ; Add/Remove Programs entry
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\bb.exe"
  WriteRegStr HKLM "${UNINSTKEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoRepair" 1

  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

Section "Add to system PATH" SecPath
  EnVar::SetHKLM
  EnVar::AddValue "Path" "$INSTDIR"
  Pop $0
  DetailPrint "EnVar AddValue Path returned: $0"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "Installs the bb command-line executable."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecPath} "Adds the install directory to the system PATH so 'bb' works from any terminal."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  EnVar::SetHKLM
  EnVar::DeleteValue "Path" "$INSTDIR"
  Pop $0

  Delete "$INSTDIR\bb.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "Software\bb"
  DeleteRegKey HKLM "${UNINSTKEY}"
SectionEnd
