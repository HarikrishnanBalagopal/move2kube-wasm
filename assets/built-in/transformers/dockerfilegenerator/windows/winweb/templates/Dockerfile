#   Copyright IBM Corporation 2021
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

{{ if .IncludeBuildStage }}
# Build Stage
FROM --platform=windows/amd64 mcr.microsoft.com/dotnet/framework/sdk:{{ .BuildStageImageTag }} AS {{ .BuildContainerName }}
WORKDIR /app
COPY . .
RUN nuget restore
RUN msbuild /p:Configuration=Release
{{- end }}

{{ if .IncludeRunStage }}
# Run Stage
FROM --platform=windows/amd64 mcr.microsoft.com/dotnet/framework/aspnet:{{ .RunStageImageTag }}
ENV DOTNET_GENERATE_ASPNET_CERTIFICATE=false
WORKDIR /inetpub/wwwroot
{{- range .Ports }}
EXPOSE {{ . }}
{{- end }}
COPY {{ if .BuildContainerName }}--from={{ .BuildContainerName }}{{ end }} {{ .CopyFrom }} ./
{{- end }}
