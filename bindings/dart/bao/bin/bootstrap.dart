import 'dart:convert';
import 'dart:io';

import 'package:archive/archive.dart';
import 'package:crypto/crypto.dart' as crypto;
import 'package:http/http.dart' as http;
import 'package:path/path.dart' as path;

const _repoOwner = 'stregato';
const _repoName = 'bao';
const _tmpDirName = 'tmp_bao';

const _platformAssets = [
  _PlatformAsset('darwin', ['macos', 'ios'], '.dylib'),
  _PlatformAsset('android', ['android'], '.so'),
  _PlatformAsset('linux', ['linux'], '.so'),
  _PlatformAsset('windows', ['windows'], '.dll'),
];
void _log(String message) => print('[bao bootstrap] $message');

Future<void> main() async {
  final rootDir = Directory.current;
  final tmpDir = Directory(path.join(rootDir.path, _tmpDirName));
  final release = await _fetchLatestRelease();
  await tmpDir.create(recursive: true);
  final downloadedAssets = <String, Directory>{};
  final client = http.Client();

  try {
    var copied = false;
    for (final asset in _platformAssets) {
      final targetDirs = asset.targetDirs
          .where((dir) => Directory(path.join(rootDir.path, dir)).existsSync())
          .toList();
      if (targetDirs.isEmpty) {
        continue;
      }

      copied = true;
      final assetDir = await _downloadAndExtractAsset(
        client: client,
        release: release,
        os: asset.osKey,
        tmpRoot: tmpDir,
        cache: downloadedAssets,
      );

      await _copyLibraries(
        assetDir: assetDir,
        targetDirs: targetDirs,
        extension: asset.libExtension,
        rootPath: rootDir.path,
        copyFrameworks: asset.osKey == 'darwin',
      );
    }

    if (!copied) {
      print('No target directories were found; nothing to copy.');
    } else {
      print('Downloaded and extracted to temporary folders.');
    }
    await _postSetup(rootDir.path);
  } finally {
    client.close();
    if (await tmpDir.exists()) {
      await tmpDir.delete(recursive: true);
    }
  }
}

Future<Map<String, dynamic>> _fetchLatestRelease() async {
  final url = Uri.parse('https://api.github.com/repos/$_repoOwner/$_repoName/releases/latest');
  final response = await http.get(url, headers: {'Accept': 'application/vnd.github.v3+json'});
  if (response.statusCode != 200) {
    throw HttpException('Failed to fetch latest release: HTTP ${response.statusCode}');
  }
  return jsonDecode(response.body) as Map<String, dynamic>;
}

Future<Directory> _downloadAndExtractAsset({
  required http.Client client,
  required Map<String, dynamic> release,
  required String os,
  required Directory tmpRoot,
  required Map<String, Directory> cache,
}) async {
  if (cache.containsKey(os)) {
    return cache[os]!;
  }

  final assetDir = Directory(path.join(tmpRoot.path, os));
  await assetDir.create(recursive: true);
  final assetName = 'bao_$os.zip';
  final assetUrl = _findAssetUrl(release, assetName);
  print('Downloading $assetName...');
  final response = await client.get(Uri.parse(assetUrl));
  if (response.statusCode != 200) {
    throw HttpException('Failed to download $assetName: HTTP ${response.statusCode}');
  }

  final archive = ZipDecoder().decodeBytes(response.bodyBytes);
  for (final file in archive) {
    final filePath = path.join(assetDir.path, file.name);
    if (file.isFile) {
      final outputFile = File(filePath);
      await outputFile.create(recursive: true);
      await outputFile.writeAsBytes(file.content as List<int>);
    } else {
      await Directory(filePath).create(recursive: true);
    }
  }

  cache[os] = assetDir;
  return assetDir;
}

String _findAssetUrl(Map<String, dynamic> release, String assetName) {
  final assets = release['assets'] as List<dynamic>? ?? [];
  for (final rawAsset in assets) {
    final asset = rawAsset as Map<String, dynamic>;
    if (asset['name'] == assetName) {
      final url = asset['browser_download_url'] as String?;
      if (url != null) {
        return url;
      }
    }
  }
  throw StateError('Release asset $assetName not found.');
}

Future<void> _copyLibraries({
  required Directory assetDir,
  required List<String> targetDirs,
  required String extension,
  required String rootPath,
  bool copyFrameworks = false,
}) async {
  final normalizedExtension = extension.toLowerCase();
  final libs = <File>[];
  await for (final entity in assetDir.list(recursive: true, followLinks: false)) {
    if (entity is File && entity.path.toLowerCase().endsWith(normalizedExtension)) {
      libs.add(entity);
    }
  }

  if (libs.isEmpty) {
    print('No $extension libraries found in ${path.basename(assetDir.path)}.');
    return;
  }

  for (final dirName in targetDirs) {
    final destination = Directory(path.join(rootPath, dirName, 'Libraries'));
    await destination.create(recursive: true);
    for (final lib in libs) {
      final destFile = File(path.join(destination.path, path.basename(lib.path)));
      await lib.copy(destFile.path);
    }
    print('Copied ${libs.length} $extension files to $dirName/Libraries.');
    if (dirName == 'android') {
      final androidJni = Directory(path.join(rootPath, 'android', 'app', 'src', 'main', 'jniLibs', 'arm64-v8a'));
      await androidJni.create(recursive: true);
      for (final lib in libs) {
        final destFile = File(path.join(androidJni.path, path.basename(lib.path)));
        await lib.copy(destFile.path);
      }
      _log('Synced ${libs.length} $extension libraries into android/app/src/main/jniLibs/arm64-v8a.');
    }
  }

  if (copyFrameworks) {
    await _copyFrameworks(assetDir, targetDirs, rootPath);
  }
}

Future<void> _copyFrameworks(Directory assetDir, List<String> targetDirs, String rootPath) async {
  final frameworks = <Directory>[];
  await for (final entity in assetDir.list(recursive: true, followLinks: false)) {
    if (entity is Directory && entity.path.endsWith('.xcframework')) {
      frameworks.add(entity);
    }
  }

  if (frameworks.isEmpty) {
    _log('No .xcframework assets found inside ${assetDir.path}.');
    return;
  }

  final eligibleTargets = targetDirs.where((dir) => dir == 'ios' || dir == 'macos');
  for (final target in eligibleTargets) {
    final frameworksRoot = Directory(path.join(rootPath, target, 'Frameworks'));
    await frameworksRoot.create(recursive: true);
    for (final framework in frameworks) {
      final destination = Directory(path.join(frameworksRoot.path, path.basename(framework.path)));
      await _copyDirectory(framework, destination);
      _log('Copied ${path.basename(framework.path)} to $target/Frameworks.');
    }
  }
}

Future<void> _copyDirectory(Directory source, Directory destination) async {
  if (await destination.exists()) {
    await destination.delete(recursive: true);
  }
  await destination.create(recursive: true);
  await for (final entity in source.list(recursive: true, followLinks: false)) {
    final relative = path.relative(entity.path, from: source.path);
    final targetPath = path.join(destination.path, relative);
    if (entity is File) {
      final file = File(targetPath);
      await file.create(recursive: true);
      await file.writeAsBytes(await entity.readAsBytes());
    } else if (entity is Directory) {
      await Directory(targetPath).create(recursive: true);
    }
  }
}

Future<void> _postSetup(String rootPath) async {
  for (final platform in ['ios', 'macos']) {
    final platformDir = Directory(path.join(rootPath, platform));
    if (!await platformDir.exists()) {
      continue;
    }
    await _ensureInfoPlist(rootPath, platform);
    final frameworkDir = Directory(path.join(rootPath, platform, 'Frameworks', 'bao.xcframework'));
    if (await frameworkDir.exists()) {
      await _ensureFrameworkInPbx(rootPath, platform);
    } else {
      _log('No bao.xcframework found for $platform; skipping Xcode project updates.');
    }
  }
  await _ensureAndroidManifest(rootPath);
}

Future<void> _ensureInfoPlist(String rootPath, String platform) async {
  final infoFile = File(path.join(rootPath, platform, 'Runner', 'Info.plist'));
  if (!await infoFile.exists()) {
    _log('Info.plist not found for $platform; cannot add permissions.');
    return;
  }
  final updated = await _updateInfoPlist(infoFile);
  if (updated) {
    _log('Updated $platform/Runner/Info.plist with network/local storage permissions.');
  } else {
    _log('$platform/Runner/Info.plist already declares the required permissions.');
  }
}

Future<bool> _updateInfoPlist(File file) async {
  var content = await file.readAsString();
  var updated = false;

  if (!content.contains('<key>NSAppTransportSecurity</key>')) {
    const block = '\n\t<key>NSAppTransportSecurity</key>\n\t<dict>\n\t\t<key>NSAllowsArbitraryLoads</key>\n\t\t<true/>\n\t</dict>\n';
    content = _insertBeforeClosingDict(content, block);
    updated = true;
  } else {
    final securityStart = content.indexOf('<key>NSAppTransportSecurity</key>');
    final dictStart = securityStart == -1 ? -1 : content.indexOf('<dict>', securityStart);
    final dictEnd = dictStart == -1 ? -1 : content.indexOf('</dict>', dictStart);
    if (dictStart != -1 && dictEnd != -1) {
      final inside = content.substring(dictStart, dictEnd);
      if (!inside.contains('<key>NSAllowsArbitraryLoads</key>')) {
        const insert = '\t\t<key>NSAllowsArbitraryLoads</key>\n\t\t<true/>\n';
        content = content.replaceRange(dictEnd, dictEnd, insert);
        updated = true;
      } else if (!inside.contains('<true/>')) {
        final keyIndex = content.indexOf('<key>NSAllowsArbitraryLoads</key>', dictStart);
        if (keyIndex != -1) {
          final valueStart = content.indexOf('<', keyIndex + 1);
          final valueEnd = valueStart == -1 ? -1 : content.indexOf('>', valueStart);
          if (valueStart != -1 && valueEnd != -1) {
            content = content.replaceRange(valueStart, valueEnd + 1, '<true/>');
            updated = true;
          }
        }
      }
    }
  }

  if (!content.contains('<key>NSLocalNetworkUsageDescription</key>')) {
    const desc = '\n\t<key>NSLocalNetworkUsageDescription</key>\n\t<string>Bao needs local network access to sync files with remote storage.</string>\n';
    content = _insertBeforeClosingDict(content, desc);
    updated = true;
  }

  if (updated) {
    await file.writeAsString(content);
  }
  return updated;
}

String _insertBeforeClosingDict(String content, String insertion) {
  final idx = content.lastIndexOf('</dict>');
  if (idx == -1) {
    return content + insertion;
  }
  return content.replaceRange(idx, idx, insertion);
}

Future<void> _ensureAndroidManifest(String rootPath) async {
  final manifestFile = File(path.join(rootPath, 'android', 'app', 'src', 'main', 'AndroidManifest.xml'));
  if (!await manifestFile.exists()) {
    _log('AndroidManifest.xml not found; skipping permission additions.');
    return;
  }
  var content = await manifestFile.readAsString();
  final permissions = [
    'android.permission.INTERNET',
    'android.permission.ACCESS_NETWORK_STATE',
  ];
  var updated = false;
  for (final permission in permissions) {
    if (!content.contains('android:name="$permission"')) {
      final insert = '    <uses-permission android:name="$permission" />\n';
      content = _insertBeforeClosingManifest(content, insert);
      updated = true;
    }
  }
  if (updated) {
    await manifestFile.writeAsString(content);
    _log('Added network permissions to android/app/src/main/AndroidManifest.xml.');
  } else {
    _log('AndroidManifest.xml already declares the required permissions.');
  }
}

String _insertBeforeClosingManifest(String content, String insertion) {
  final idx = content.lastIndexOf('</manifest>');
  if (idx == -1) {
    return content + insertion;
  }
  return content.replaceRange(idx, idx, insertion);
}

Future<void> _ensureFrameworkInPbx(String rootPath, String platform) async {
  final pbxPath = path.join(rootPath, platform, 'Runner.xcodeproj', 'project.pbxproj');
  final pbxFile = File(pbxPath);
  if (!await pbxFile.exists()) {
    _log('Runner.xcodeproj not found for $platform; skipping framework integration.');
    return;
  }
  var content = await pbxFile.readAsString();
  if (content.contains('bao.xcframework')) {
    _log('Runner project for $platform already references bao.xcframework.');
    return;
  }
  final fileRefId = _generatePbxId('$platform-bao-framework-file');
  final buildFileId = _generatePbxId('$platform-bao-framework-build');
  content = _insertPbxEntry(content, 'PBXFileReference', '\t$fileRefId /* bao.xcframework */ = {\n\t\tisa = PBXFileReference;\n\t\tlastKnownFileType = wrapper.xcframework;\n\t\tname = bao.xcframework;\n\t\tpath = Frameworks/bao.xcframework;\n\t\tsourceTree = "<group>";\n\t};\n');
  content = _insertPbxEntry(content, 'PBXBuildFile', '\t$buildFileId /* bao.xcframework in Frameworks */ = {\n\t\tisa = PBXBuildFile;\n\t\tfileRef = $fileRefId /* bao.xcframework */;\n\t};\n');
  content = _appendToChildrenList(content, 'Frameworks', '$fileRefId /* bao.xcframework */');
  content = _appendToBuildPhase(content, 'PBXFrameworksBuildPhase', '$buildFileId /* bao.xcframework in Frameworks */');
  content = _appendToEmbedPhase(content, '$buildFileId /* bao.xcframework in Frameworks */');
  await pbxFile.writeAsString(content);
  _log('Injected bao.xcframework references into $platform Runner project.');
}

String _insertPbxEntry(String content, String sectionName, String entry) {
  final endMarker = '/* End $sectionName section */';
  final endIndex = content.indexOf(endMarker);
  if (endIndex == -1) {
    _log('Unable to locate $sectionName section in pbx project.');
    return content;
  }
  return content.replaceRange(endIndex, endIndex, entry);
}

String _appendToChildrenList(String content, String groupName, String entry) {
  final groupMarker = '/* $groupName */ = {';
  final groupIndex = content.indexOf(groupMarker);
  if (groupIndex == -1) {
    _log('Unable to find $groupName group in pbx project.');
    return content;
  }
  final childrenMarker = 'children = (';
  final childrenIndex = content.indexOf(childrenMarker, groupIndex);
  if (childrenIndex == -1) {
    return content;
  }
  final insertPoint = content.indexOf(');', childrenIndex);
  if (insertPoint == -1) {
    return content;
  }
  return content.replaceRange(insertPoint, insertPoint, '\t\t\t$entry,\n');
}

String _appendToBuildPhase(String content, String sectionName, String entry) {
  final phaseMarker = '/* $sectionName */ = {';
  final phaseIndex = content.indexOf(phaseMarker);
  if (phaseIndex == -1) {
    _log('Unable to locate $sectionName in pbx project.');
    return content;
  }
  final filesMarker = 'files = (';
  final filesIndex = content.indexOf(filesMarker, phaseIndex);
  if (filesIndex == -1) {
    return content;
  }
  final insertPoint = content.indexOf(');', filesIndex);
  if (insertPoint == -1) {
    return content;
  }
  return content.replaceRange(insertPoint, insertPoint, '\t\t\t$entry,\n');
}

String _appendToEmbedPhase(String content, String entry) {
  const embedMarker = 'name = "Embed Frameworks";';
  final embedIndex = content.indexOf(embedMarker);
  if (embedIndex == -1) {
    _log('Embed Frameworks phase not found in pbx project.');
    return content;
  }
  const filesMarker = 'files = (';
  final filesIndex = content.indexOf(filesMarker, embedIndex);
  if (filesIndex == -1) {
    return content;
  }
  final insertPoint = content.indexOf(');', filesIndex);
  if (insertPoint == -1) {
    return content;
  }
  return content.replaceRange(insertPoint, insertPoint, '\t\t\t$entry,\n');
}

String _generatePbxId(String seed) {
  final digest = crypto.sha1.convert(utf8.encode(seed));
  return digest.toString().substring(0, 24).toUpperCase();
}

class _PlatformAsset {
  final String osKey;
  final List<String> targetDirs;
  final String libExtension;

  const _PlatformAsset(this.osKey, this.targetDirs, this.libExtension);
}
