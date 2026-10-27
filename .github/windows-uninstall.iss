[Code]
procedure RemoveWhiskrData(Directory: String; var FailedPaths: String);
begin
	while DirExists(Directory) do
	begin
		if DelTree(Directory, True, True, True) then
			Exit;

		Log('Could not completely remove Whiskr data: ' + Directory);

		if SuppressibleMsgBox('Could not completely remove:' + #13#10 + Directory + #13#10#13#10 + 'Close Whiskr and any programs using this folder, then choose Retry. Choose Cancel to leave any remaining files in this folder.', mbError, MB_RETRYCANCEL, IDCANCEL) <> IDRETRY then
		begin
			FailedPaths := FailedPaths + #13#10 + Directory;
			Exit;
		end;
	end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
	FailedPaths: String;
begin
	if (CurUninstallStep <> usPostUninstall) or UninstallSilent then
		Exit;

	if SuppressibleMsgBox('Also permanently remove all saved Whiskr data for your Windows user?' + #13#10#13#10 + 'This includes configuration files, API tokens, settings, chat history, browser data, caches and logs in Whiskr folders under AppData\Local and AppData\Roaming.' + #13#10#13#10 + 'Close Whiskr before continuing. This cannot be undone.' + #13#10#13#10 + 'Choose No to keep your data for a future installation.', mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) <> IDYES then
		Exit;

	FailedPaths := '';
	RemoveWhiskrData(ExpandConstant('{userappdata}\whiskr'), FailedPaths);
	RemoveWhiskrData(ExpandConstant('{localappdata}\whiskr'), FailedPaths);
	RemoveWhiskrData(ExpandConstant('{userappdata}\whiskr_desktop'), FailedPaths);
	RemoveWhiskrData(ExpandConstant('{localappdata}\whiskr_desktop'), FailedPaths);

	// Glaze uses the executable's filename, including .exe, as its WebView2 data folder.
	RemoveWhiskrData(ExpandConstant('{userappdata}\{#MyAppExeName}'), FailedPaths);
	RemoveWhiskrData(ExpandConstant('{localappdata}\{#MyAppExeName}'), FailedPaths);

	if FailedPaths <> '' then
		SuppressibleMsgBox('Some Whiskr data could not be removed. You can remove these folders manually after closing programs that use them:' + #13#10 + FailedPaths, mbError, MB_OK, IDOK);
end;
