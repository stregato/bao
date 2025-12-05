import 'dart:io';

void copyLibraries() {
  // Define source and destination directories
  final sourceDir = Directory('../../../build');
  final destinationDir = Directory('lib/assets');

  // Check if source directory exists
  if (!sourceDir.existsSync()) {
    print('Source directory does not exist: ${sourceDir.path}');
    return;
  }

  // Ensure the destination directory exists
  if (!destinationDir.existsSync()) {
    destinationDir.createSync(recursive: true);
    print('Created destination directory: ${destinationDir.path}');
  }

  // List of allowed file extensions for libraries
  final libraryExtensions = ['.so', '.dylib', '.dll'];

  // Function to check if a file has a valid library extension
  bool isLibraryFile(String fileName) {
    return libraryExtensions.any((ext) => fileName.endsWith(ext));
  }

  // Recursively copy all valid library files
  for (var entity in sourceDir.listSync(recursive: true)) {
    if (entity is File && isLibraryFile(entity.path)) {
      // Get the relative path of the file from the source directory
      final relativePath = entity.path.replaceFirst(sourceDir.path, '');
      
      // Define the destination path
      final destinationPath = destinationDir.path + relativePath;

      // Ensure destination directory exists
      final destinationFile = File(destinationPath);
      destinationFile.createSync(recursive: true);

      // Copy the file
      entity.copySync(destinationPath);
      print('Copied ${entity.path} to $destinationPath');
    }
  }

  print('All library files copied successfully!');
}

void main() {
  copyLibraries();
}
