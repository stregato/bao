package ink.francesco.bao;

import java.io.File;
import java.net.URL;

import com.sun.jna.Library;
import com.sun.jna.Native;
import com.sun.jna.Platform;


public interface BaoLibrary extends Library {

    Result bao_setLogLevel(String level);
    Result bao_core_setHttpLog(String addr);
    Result bao_core_getRecentLog(long n);
    Result bao_snapshot();

    Result bao_security_newPrivateID();
    Result bao_security_newKeyPair();
    Result bao_security_publicID(String privateID);
    Result bao_security_decodePrivateID(String encoded);
    Result bao_security_decodePublicID(String encoded);

    Result bao_security_ecEncrypt(String pub, Data data);
    Result bao_security_ecDecrypt(String priv, Data data);
    Result bao_security_aesEncrypt(String key, Data data, Data iv);
    Result bao_security_aesDecrypt(String key, Data data, Data iv);

    Result bao_db_open(String driver, String path, String ddl);
    Result bao_db_close(long dbH);
    Result bao_db_query(long dbH, String key, String argsJson);
    Result bao_db_exec(long dbH, String key, String argsJson);
    Result bao_db_fetch(long dbH, String key, String argsJson, int maxRows);
    Result bao_db_fetch_one(long dbH, String key, String argsJson);

    Result bao_store_open(String configJson);
    Result bao_store_close(long storeH);
    Result bao_store_readDir(long storeH, String dir, String filterJson);
    Result bao_store_stat(long storeH, String path);
    Result bao_store_delete(long storeH, String path);

    Result bao_vault_create(String realm, String identity, long storeH, long dbH, String settingsJson);
    Result bao_vault_open(String realm, String identity, String author, long storeH, long dbH);
    Result bao_vault_close(long baoH);
    Result bao_vault_syncAccess(long baoH, long options, String changesJson);
    Result bao_vault_getAccess(long baoH, String userId);
    Result bao_vault_getGroups(long baoH, String userId);
    Result bao_listGroups(long baoH);
    Result bao_vault_waitFiles(long baoH, String fileIdsJson);
    Result bao_vault_sync(long baoH, String groupsJson);
    Result bao_vault_setAttribute(long baoH, long options, String name, String value);
    Result bao_vault_getAttribute(long baoH, String name, String author);
    Result bao_vault_getAttributes(long baoH, String author);
    Result bao_vault_readDir(long baoH, String dir, long since, long fromId, int limit);
    Result bao_vault_stat(long baoH, String name);
    Result bao_vault_read(long baoH, String name, String dest, long options);
    Result bao_vault_write(long baoH, String dest, String src, Data attrs, long options);
    Result bao_vault_delete(long baoH, String name, long options);
    Result bao_vault_allocatedSize(long baoH);

    Result bao_replica_open(long baoH, int dbHandle);
    Result bao_replica_exec(long layerH, String query, String args);
    Result bao_replica_query(long layerH, String query, String args);
    Result bao_replica_fetch(long rowsH, String dest, String args, int limit);
    Result bao_replica_fetchOne(long rowsH, String dest, String args);
    Result bao_replica_next(long rowsH);
    Result bao_replica_current(long rowsH);
    Result bao_replica_closeRows(long rowsH);
    Result bao_replica_sync(long layerH);
    Result bao_replica_cancel(long layerH);

    Result bao_mailbox_send(long baoH, String dir, String group, String messageJson);
    Result bao_mailbox_receive(long baoH, String dir, long since, long fromId);
    Result bao_mailbox_download(long baoH, String dir, String messageJson, int attachmentIndex, String dest);
    
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
