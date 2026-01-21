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

publish:
	dotnet publish --configuration Release --output ./publish

publish-to-core:
	dotnet publish --configuration Release --output ../wox.core/resource/ui/wpf
