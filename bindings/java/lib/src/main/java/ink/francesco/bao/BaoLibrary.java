package ink.francesco.bao;

import java.io.File;
import java.net.URL;

import com.sun.jna.Library;
import com.sun.jna.Native;
import com.sun.jna.Platform;


public interface BaoLibrary extends Library {

    Result bao_setLogLevel(String level);
    Result bao_setHttpLog(String addr);
    Result bao_getRecentLog(long n);
    Result bao_snapshot();

    Result bao_newPrivateID();
    Result bao_publicID(String privateID);
    Result bao_decodeID(String encoded);

    Result bao_ecEncrypt(String pub, Data data);
    Result bao_ecDecrypt(String priv, Data data);
    Result bao_aesEncrypt(String key, Data data, Data iv);
    Result bao_aesDecrypt(String key, Data data, Data iv);

    Result bao_openDB(String path);
    Result bao_closeDB(long dbH);
    Result bao_dbQuery(long dbH, String key, String argsJson);
    Result bao_dbExec(long dbH, String key, String argsJson);
    Result bao_dbFetch(long dbH, String key, String argsJson, int maxRows);
    Result bao_dbFetchOne(long dbH, String key, String argsJson);

    Result bao_create(long dbH, String identity, String url, String settingsJson);
    Result bao_open(long dbH, String identity, String url, String author);
    Result bao_close(long baoH);
    Result bao_syncAccess(long baoH, long options, String changesJson);
    Result bao_getAccess(long baoH, String groupName);
    Result bao_getGroups(long baoH, String userId);
    Result bao_listGroups(long baoH);
    Result bao_waitFiles(long baoH, String fileIdsJson);
    Result bao_sync(long baoH, String groupsJson);
    Result bao_setAttribute(long baoH, long options, String name, String value);
    Result bao_getAttribute(long baoH, String name, String author);
    Result bao_getAttributes(long baoH, String author);
    Result bao_readDir(long baoH, String dir, long since, long fromId, int limit);
    Result bao_stat(long baoH, String name);
    Result bao_read(long baoH, String name, String dest, long options);
    Result bao_write(long baoH, String dest, String src, String group, Data attrs, long options);
    Result bao_delete(long baoH, String name, long options);
    Result bao_allocatedSize(long baoH);

    Result baoql_layer(long baoH, String groupName, int dbHandle);
    Result baoql_exec(long layerH, String query, String args);
    Result baoql_query(long layerH, String query, String args);
    Result baoql_fetch(long rowsH, String dest, String args, int limit);
    Result baoql_fetchOne(long rowsH, String dest, String args);
    Result baoql_next(long rowsH);
    Result baoql_current(long rowsH);
    Result baoql_closeRows(long rowsH);
    Result baoql_sync_tables(long layerH);
    Result baoql_cancel(long layerH);

    Result mailbox_send(long baoH, String dir, String group, String messageJson);
    Result mailbox_receive(long baoH, String dir, long since, long fromId);
    Result mailbox_download(long baoH, String dir, String messageJson, int attachmentIndex, String dest);
    
    static BaoLibrary loadLibrary() {
        String base = "bao";
        String ext = "";
        String osFolder = "";
        String arch = System.getProperty("os.arch");
        String suffix = arch.equals("amd64") ? "amd64" : "arm64";
    
        // Determine the platform-specific folder and library extension
        if (Platform.isWindows()) {
            osFolder = "windows";
            ext = ".dll";
        } else if (Platform.isMac()) {
            osFolder = "darwin";
            base = "libbao_" + suffix;
            ext = ".dylib";
        } else if (Platform.isLinux()) {
            osFolder = "linux";
            base = "libbao_" + suffix;
            ext = ".so";
        } else if (Platform.isAndroid()) {
            osFolder = "android";
            base = "libbao_" + suffix;
            ext = ".so";
        }

        if (Platform.isWindows()) {
            base = "bao_" + suffix;
        }
    
        // Construct the library name for JNA to load
        String resourceLibName = base + ext;
        BaoLibrary libraryInstance = null;

        try {
            // First attempt to load the library via JNA (without specifying full path)
            libraryInstance = (BaoLibrary) Native.load(base, BaoLibrary.class);
        } catch (UnsatisfiedLinkError e) {
            // Calculate fallback path relative to the location of the class
            URL classLocation = BaoLibrary.class.getProtectionDomain().getCodeSource().getLocation();
            File classFile = new File(classLocation.getPath());
            File classDir = classFile.isDirectory() ? classFile : classFile.getParentFile();

            // Now calculate the path to the build directory relative to the class location
            File buildDir = new File(classDir, "../../../../build/" + osFolder + "/" + resourceLibName);
            if (buildDir.exists()) {
                // Load the library using the full path
                libraryInstance = (BaoLibrary) Native.load(buildDir.getAbsolutePath(), BaoLibrary.class);
            } else {
                throw new RuntimeException("Failed to load native library from both JAR and ../../build directory", e);
            }
        }

        // Synchronize the loaded library
        return (BaoLibrary) Native.synchronizedLibrary(libraryInstance);
    }

    BaoLibrary instance = BaoLibrary.loadLibrary();
}
