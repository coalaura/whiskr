[Code]
var
	ConfigPage: TInputOptionWizardPage;
	ProviderPage: TInputOptionWizardPage;
	CredentialsPage: TInputQueryWizardPage;
	TitlePage: TWizardPage;
	TitleEnabled: TNewCheckBox;
	TitleModelLabel: TNewStaticText;
	TitleModel: TNewEdit;
	ConfigPath: String;
	PreviousTitleModel: String;

function ProviderType: String;
begin
	if ProviderPage.SelectedValueIndex = 1 then
		Result := 'openai'
	else
		Result := 'openrouter';
end;

function DefaultBaseURL: String;
begin
	if ProviderType = 'openai' then
		Result := 'https://api.openai.com/v1'
	else
		Result := 'https://openrouter.ai/api/v1';
end;

function DefaultTitleModel: String;
begin
	if ProviderType = 'openai' then
		Result := 'gpt-6-luna'
	else
		Result := 'google/gemini-3.1-flash-lite';
end;

function ConfigureRequested: Boolean;
begin
	Result := not WizardSilent and not FileExists(ConfigPath) and ConfigPage.Values[0];
end;

procedure TitleEnabledClick(Sender: TObject);
begin
	TitleModel.Enabled := TitleEnabled.Checked;
	TitleModelLabel.Enabled := TitleEnabled.Checked;
end;

procedure InitializeWizard;
begin
	ConfigPath := ExpandConstant('{userappdata}\whiskr\config.yml');
	ConfigPage := CreateInputOptionPage(wpSelectTasks, 'Configure Whiskr', 'Optionally prepare your configuration before the first launch.', 'Save a basic config.yml to ' + ConfigPath + '. You can edit this file later. If you skip this step, Whiskr will create and open the config on first launch.', False, False);
	ConfigPage.Add('Configure Whiskr now');
	ConfigPage.Values[0] := False;

	ProviderPage := CreateInputOptionPage(ConfigPage.ID, 'API provider', 'Choose the API Whiskr should use.', 'Provider type:', True, False);
	ProviderPage.Add('OpenRouter');
	ProviderPage.Add('OpenAI / OpenAI-compatible endpoint');
	ProviderPage.SelectedValueIndex := 0;

	CredentialsPage := CreateInputQueryPage(ProviderPage.ID, 'API connection', 'Enter the credentials for your selected provider.', 'The token is saved in your local config.yml. Leave the base URL blank to use the provider default.');
	CredentialsPage.Add('OpenRouter API token:', True);
	CredentialsPage.Add('Base URL (optional):', False);

	TitlePage := CreateCustomPage(CredentialsPage.ID, 'Chat titles', 'Choose whether Whiskr should generate titles automatically.');
	TitleEnabled := TNewCheckBox.Create(TitlePage);
	TitleEnabled.Parent := TitlePage.Surface;
	TitleEnabled.Width := TitlePage.SurfaceWidth;
	TitleEnabled.Caption := 'Enable automatic chat titles';
	TitleEnabled.Checked := True;
	TitleEnabled.OnClick := @TitleEnabledClick;

	TitleModelLabel := TNewStaticText.Create(TitlePage);
	TitleModelLabel.Parent := TitlePage.Surface;
	TitleModelLabel.Top := TitleEnabled.Top + TitleEnabled.Height + ScaleY(20);
	TitleModelLabel.Width := TitlePage.SurfaceWidth;
	TitleModelLabel.AutoSize := False;
	TitleModelLabel.Height := ScaleY(36);
	TitleModelLabel.WordWrap := True;
	TitleModelLabel.Caption := 'Title model (must be available from your endpoint and support structured output):';

	TitleModel := TNewEdit.Create(TitlePage);
	TitleModel.Parent := TitlePage.Surface;
	TitleModel.Top := TitleModelLabel.Top + TitleModelLabel.Height + ScaleY(4);
	TitleModel.Width := TitlePage.SurfaceWidth;
	TitleModel.Text := DefaultTitleModel;
	PreviousTitleModel := TitleModel.Text;
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
	Result := False;

	if PageID = ConfigPage.ID then
		Result := WizardSilent or FileExists(ConfigPath)
	else if (PageID = ProviderPage.ID) or (PageID = CredentialsPage.ID) or (PageID = TitlePage.ID) then
		Result := not ConfigureRequested;
end;

procedure CurPageChanged(CurPageID: Integer);
begin
	if CurPageID <> CredentialsPage.ID then
		Exit;

	if ProviderType = 'openai' then
		CredentialsPage.PromptLabels[0].Caption := 'OpenAI-compatible API token:'
	else
		CredentialsPage.PromptLabels[0].Caption := 'OpenRouter API token:';

	CredentialsPage.PromptLabels[1].Caption := 'Base URL (optional; default: ' + DefaultBaseURL + '):';

	if (TitleModel.Text = PreviousTitleModel) or (Trim(TitleModel.Text) = '') then
		TitleModel.Text := DefaultTitleModel;

	PreviousTitleModel := DefaultTitleModel;
end;

function ValidConfigValue(const Value: String): Boolean;
var
	Index: Integer;
begin
	Result := False;

	for Index := 1 to Length(Value) do
	begin
		if (Ord(Value[Index]) < 32) or ((Ord(Value[Index]) >= 127) and (Ord(Value[Index]) <= 159)) or (Ord(Value[Index]) = 8232) or (Ord(Value[Index]) = 8233) then
			Exit;
	end;

	Result := True;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
	BaseURL: String;
begin
	Result := True;

	if not ConfigureRequested then
		Exit;

	if CurPageID = CredentialsPage.ID then
	begin
		if (Trim(CredentialsPage.Values[0]) = '') or not ValidConfigValue(CredentialsPage.Values[0]) then
		begin
			MsgBox('Enter an API token, or go back and skip configuration.', mbError, MB_OK);
			Result := False;
			Exit;
		end;

		BaseURL := Lowercase(Trim(CredentialsPage.Values[1]));

		if not ValidConfigValue(CredentialsPage.Values[1]) or ((BaseURL <> '') and (Copy(BaseURL, 1, 7) <> 'http://') and (Copy(BaseURL, 1, 8) <> 'https://')) then
		begin
			MsgBox('Enter an HTTP or HTTPS base URL, or leave it blank for the provider default.', mbError, MB_OK);
			Result := False;
		end;
	end;

	if (CurPageID = TitlePage.ID) and TitleEnabled.Checked then
	begin
		if (Trim(TitleModel.Text) = '') or (Trim(TitleModel.Text) = '-') or not ValidConfigValue(TitleModel.Text) then
		begin
			MsgBox('Enter a title model, or disable automatic chat titles.', mbError, MB_OK);
			Result := False;
		end;
	end;
end;

function QuoteYAML(const Value: String): String;
begin
	Result := Value;
	StringChangeEx(Result, #39, #39 + #39, True);
	Result := #39 + Result + #39;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
	Lines: TArrayOfString;
	BaseURL: String;
	Model: String;
begin
	if (CurStep <> ssPostInstall) or not ConfigureRequested then
		Exit;

	BaseURL := Trim(CredentialsPage.Values[1]);

	if BaseURL = '' then
		BaseURL := DefaultBaseURL;

	Model := '-';

	if TitleEnabled.Checked then
		Model := Trim(TitleModel.Text);

	SetArrayLength(Lines, 8);
	Lines[0] := 'tokens:';
	Lines[1] := '  ' + ProviderType + ': ' + QuoteYAML(Trim(CredentialsPage.Values[0]));
	Lines[2] := 'llm:';
	Lines[3] := '  api: ' + ProviderType;
	Lines[4] := '  base-url: ' + QuoteYAML(BaseURL);
	Lines[5] := 'models:';
	Lines[6] := '  title-model: ' + QuoteYAML(Model);
	Lines[7] := '';

	if not ForceDirectories(ExtractFileDir(ConfigPath)) then
		RaiseException('Could not create the Whiskr configuration directory: ' + ExtractFileDir(ConfigPath));

	if not SaveStringsToUTF8File(ConfigPath, Lines, False) then
		RaiseException('Could not save the Whiskr configuration: ' + ConfigPath);
end;
