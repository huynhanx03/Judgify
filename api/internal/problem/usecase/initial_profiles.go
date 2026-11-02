package usecase

import (
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// initialRuntimeTemplates returns the four code-owned launch command
// templates. Artifact identities remain empty until an audited release
// manifest binds them.
func initialRuntimeTemplates() []entity.RuntimeProfile {
	return []entity.RuntimeProfile{
		{
			ID:  uuid.MustParse("11111111-1111-4111-8111-111111111117"),
			Key: entity.RuntimeKeyCPP17, Language: "cpp", DisplayName: "C++17",
			ProfileVersion: "2026.07.1", CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
			ImageDigest:      "",
			CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
			Compile: entity.CommandTemplate{Executable: "/usr/bin/g++", Arguments: []entity.CommandArgument{
				{Kind: entity.ArgumentLiteral, Value: "-std=c++17"},
				{Kind: entity.ArgumentLiteral, Value: "-O2"},
				{Kind: entity.ArgumentLiteral, Value: "-pipe"},
				{Kind: entity.ArgumentWorkerPath, Value: "source_cpp17"},
				{Kind: entity.ArgumentLiteral, Value: "-o"},
				{Kind: entity.ArgumentWorkerPath, Value: "artifact_executable"},
			}},
			Run:                   entity.CommandTemplate{Executable: "/workspace/artifact/main"},
			DefaultCompileTimeout: 15 * time.Second,
			ManifestVersion:       "cpp17-artifact.v1",
			ManifestChecksum:      "",
			Artifacts:             []entity.RuntimeArtifact{{Path: "main", Executable: true, MaximumBytes: 32 << 20, MaximumFiles: 1, Required: true}},
			Workspace: entity.RuntimeWorkspace{
				SourcePath: "source/main.cpp",
				Values: []entity.RuntimeWorkspaceValue{
					{Name: "source_cpp17", Path: "/workspace/source/main.cpp"},
					{Name: "artifact_executable", Path: "/workspace/artifact/main"},
				},
			},
			Active: true,
		},
		{
			ID:  uuid.MustParse("22222222-2222-4222-8222-222222222223"),
			Key: entity.RuntimeKeyPython3, Language: "python", DisplayName: "Python 3",
			ProfileVersion: "2026.07.1", CompilerVersion: "cpython-3.12.14", RuntimeBootstrapVersion: "cpython-3.12.14",
			ImageDigest:      "",
			CompileCommandID: "python3.compile.v1", RunCommandID: "python3.run.v1",
			Compile: entity.CommandTemplate{Executable: "/usr/bin/python3", Arguments: []entity.CommandArgument{
				{Kind: entity.ArgumentLiteral, Value: "-I"},
				{Kind: entity.ArgumentLiteral, Value: "-m"},
				{Kind: entity.ArgumentLiteral, Value: "py_compile"},
				{Kind: entity.ArgumentWorkerPath, Value: "source_python3"},
			}},
			Run: entity.CommandTemplate{Executable: "/usr/bin/python3", Arguments: []entity.CommandArgument{
				{Kind: entity.ArgumentLiteral, Value: "-I"},
				{Kind: entity.ArgumentWorkerPath, Value: "artifact_python3"},
			}},
			DefaultCompileTimeout: 10 * time.Second,
			ManifestVersion:       "python3-artifact.v1",
			ManifestChecksum:      "",
			Artifacts: []entity.RuntimeArtifact{
				{Path: "main.py", MaximumBytes: 1 << 20, MaximumFiles: 1, Required: true},
				{Path: "__pycache__/*.pyc", MaximumBytes: 4 << 20, MaximumFiles: 1},
			},
			Workspace: entity.RuntimeWorkspace{
				SourcePath: "artifact/main.py",
				Values: []entity.RuntimeWorkspaceValue{
					{Name: "source_python3", Path: "/workspace/artifact/main.py"},
					{Name: "artifact_python3", Path: "/workspace/artifact/main.py"},
				},
			},
			Active: true,
		},
		{
			ID:  uuid.MustParse("33333333-3333-4333-8333-333333333317"),
			Key: entity.RuntimeKeyJava17, Language: "java", DisplayName: "Java 17",
			ProfileVersion: "2026.07.1", CompilerVersion: "javac-17.0.20", RuntimeBootstrapVersion: "openjdk-17.0.20",
			ImageDigest:      "",
			CompileCommandID: "java17.compile.v1", RunCommandID: "java17.run.v1",
			Compile: entity.CommandTemplate{Executable: "/usr/bin/javac", Arguments: []entity.CommandArgument{
				{Kind: entity.ArgumentLiteral, Value: "-encoding"},
				{Kind: entity.ArgumentLiteral, Value: "UTF-8"},
				{Kind: entity.ArgumentLiteral, Value: "-d"},
				{Kind: entity.ArgumentWorkerPath, Value: "artifact_java17"},
				{Kind: entity.ArgumentWorkerPath, Value: "source_java17"},
			}},
			Run: entity.CommandTemplate{Executable: "/usr/bin/java", Arguments: []entity.CommandArgument{
				{Kind: entity.ArgumentLiteral, Value: "-cp"},
				{Kind: entity.ArgumentWorkerPath, Value: "artifact_java17"},
				{Kind: entity.ArgumentLiteral, Value: "Main"},
			}},
			DefaultCompileTimeout: 15 * time.Second,
			ManifestVersion:       "java17-artifact.v1",
			ManifestChecksum:      "",
			Artifacts:             []entity.RuntimeArtifact{{Path: "*.class", MaximumBytes: 16 << 20, MaximumFiles: 128, Required: true}},
			Workspace: entity.RuntimeWorkspace{
				SourcePath: "source/Main.java",
				Values: []entity.RuntimeWorkspaceValue{
					{Name: "artifact_java17", Path: "/workspace/artifact"},
					{Name: "source_java17", Path: "/workspace/source/Main.java"},
				},
			},
			Active: true,
		},
		{
			ID:  uuid.MustParse("44444444-4444-4444-8444-444444444440"),
			Key: entity.RuntimeKeyGo, Language: "go", DisplayName: "Go",
			ProfileVersion: "2026.07.1", CompilerVersion: "go1.25.10", RuntimeBootstrapVersion: "go-module-workspace.v1",
			ImageDigest:      "",
			CompileCommandID: "go.compile.v1", RunCommandID: "go.run.v1",
			Compile: entity.CommandTemplate{
				Executable:       "/usr/bin/go",
				WorkingDirectory: entity.CommandArgument{Kind: entity.ArgumentWorkerPath, Value: "prepared_go_module_workspace"},
				Arguments: []entity.CommandArgument{
					{Kind: entity.ArgumentLiteral, Value: "build"},
					{Kind: entity.ArgumentLiteral, Value: "-trimpath"},
					{Kind: entity.ArgumentLiteral, Value: "-mod=vendor"},
					{Kind: entity.ArgumentLiteral, Value: "-o"},
					{Kind: entity.ArgumentWorkerPath, Value: "artifact_executable"},
					{Kind: entity.ArgumentLiteral, Value: "./cmd/submission"},
				},
				Environment: []entity.EnvironmentVariable{
					{Name: "GOPROXY", Value: "off"},
					{Name: "GOSUMDB", Value: "off"},
					{Name: "GOFLAGS", Value: "-mod=vendor"},
				},
			},
			Run:                   entity.CommandTemplate{Executable: "/workspace/artifact/main"},
			DefaultCompileTimeout: 15 * time.Second,
			ManifestVersion:       "go-artifact.v1",
			ManifestChecksum:      "",
			Artifacts:             []entity.RuntimeArtifact{{Path: "main", Executable: true, MaximumBytes: 32 << 20, MaximumFiles: 1, Required: true}},
			Workspace: entity.RuntimeWorkspace{
				SourcePath: "source/go-module/cmd/submission/main.go",
				Values: []entity.RuntimeWorkspaceValue{
					{Name: "prepared_go_module_workspace", Path: "/workspace/source/go-module"},
					{Name: "artifact_executable", Path: "/workspace/artifact/main"},
				},
				TrustedFiles: []entity.RuntimeWorkspaceFile{{
					Path:    "source/go-module/go.mod",
					Content: "module judgify.local/submission\n\ngo 1.25\n",
				}},
			},
			Active: true,
		},
	}
}

func initialCheckerTemplates() []entity.CheckerProfile {
	return []entity.CheckerProfile{
		{
			ID:  uuid.MustParse("55555555-5555-4555-8555-555555555551"),
			Key: entity.CheckerKeyExact, Version: "v1",
			ImplementationChecksum: "",
			ConfigKind:             entity.CheckerConfigNone, Active: true,
		},
		{
			ID:  uuid.MustParse("66666666-6666-4666-8666-666666666662"),
			Key: entity.CheckerKeyToken, Version: "v1",
			ImplementationChecksum: "",
			ConfigKind:             entity.CheckerConfigNone, Active: true,
		},
		{
			ID:  uuid.MustParse("77777777-7777-4777-8777-777777777773"),
			Key: entity.CheckerKeyFloat, Version: "v1",
			ImplementationChecksum: "",
			ConfigKind:             entity.CheckerConfigFloatTolerance, Active: true,
		},
	}
}
