.PHONY: build clean restore run

build: restore
	dotnet build --configuration Release

restore:
	dotnet restore

clean:
	dotnet clean
	rm -rf bin obj publish

run:
	dotnet run

# 发布自包含版本（包含运行时，用户无需安装 .NET）
publish:
	dotnet publish --configuration Release --output ./publish

# 发布到 wox.core 资源目录
publish-to-core:
	dotnet publish --configuration Release --output ../wox.core/resource/ui/wpf
