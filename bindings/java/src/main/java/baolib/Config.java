package baolib;

/**
 * Configuration for a Vault instance.
 * 
 * All time-based parameters are in milliseconds.
 */
public class Config {
    /** Sync relay server URL (e.g., wss://sync-relay.example.com). Leave empty to disable. */
    private String syncRelay;

    /** How long data is kept (in milliseconds) */
    private long retention;

    /** Maximum allowed store size in bytes */
    private long maxStorage;

    /** Time duration of each batch segment (in milliseconds) */
    private long segmentInterval;

    /** Minimum time between two sync operations (default: 5000ms) */
    private long syncCooldown;

    /** Maximum time to wait for I/O operations (default: 600000ms - 10 minutes) */
    private long waitTimeout;

    /** How often to sync files (default: 600000ms - 10 minutes) */
    private long filesSyncPeriod;

    /** How often to run housekeeping (default: 3600000ms - 1 hour) */
    private long cleanupPeriod;

    /** How often to sync the blockchain (default: 600000ms - 10 minutes) */
    private long blockChainSyncPeriod;

    /** Maximum number of concurrent I/O operations (default: 10) */
    private long ioThrottle;

    /**
     * Create a new Config with default values
     */
    public Config() {
        this.syncRelay = "";
        this.retention = 0;
        this.maxStorage = 0;
        this.segmentInterval = 0;
        this.syncCooldown = 0;
        this.waitTimeout = 0;
        this.filesSyncPeriod = 0;
        this.cleanupPeriod = 0;
        this.blockChainSyncPeriod = 0;
        this.ioThrottle = 0;
    }

    // Getters
    public String getSyncRelay() { return syncRelay; }
    public long getRetention() { return retention; }
    public long getMaxStorage() { return maxStorage; }
    public long getSegmentInterval() { return segmentInterval; }
    public long getSyncCooldown() { return syncCooldown; }
    public long getWaitTimeout() { return waitTimeout; }
    public long getFilesSyncPeriod() { return filesSyncPeriod; }
    public long getCleanupPeriod() { return cleanupPeriod; }
    public long getBlockChainSyncPeriod() { return blockChainSyncPeriod; }
    public long getIoThrottle() { return ioThrottle; }

    // Setters
    public Config setSyncRelay(String syncRelay) {
        this.syncRelay = syncRelay != null ? syncRelay : "";
        return this;
    }

    public Config setRetention(long retention) {
        this.retention = retention;
        return this;
    }

    public Config setMaxStorage(long maxStorage) {
        this.maxStorage = maxStorage;
        return this;
    }

    public Config setSegmentInterval(long segmentInterval) {
        this.segmentInterval = segmentInterval;
        return this;
    }

    public Config setSyncCooldown(long syncCooldown) {
        this.syncCooldown = syncCooldown;
        return this;
    }

    public Config setWaitTimeout(long waitTimeout) {
        this.waitTimeout = waitTimeout;
        return this;
    }

    public Config setFilesSyncPeriod(long filesSyncPeriod) {
        this.filesSyncPeriod = filesSyncPeriod;
        return this;
    }

    public Config setCleanupPeriod(long cleanupPeriod) {
        this.cleanupPeriod = cleanupPeriod;
        return this;
    }

    public Config setBlockChainSyncPeriod(long blockChainSyncPeriod) {
        this.blockChainSyncPeriod = blockChainSyncPeriod;
        return this;
    }

    public Config setIoThrottle(long ioThrottle) {
        this.ioThrottle = ioThrottle;
        return this;
    }

    /**
     * Convert Config to a JSON-compatible Map for C API
     */
    public java.util.Map<String, Object> toMap() {
        java.util.Map<String, Object> map = new java.util.HashMap<>();
        if (syncRelay != null && !syncRelay.isEmpty()) {
            map.put("syncRelay", syncRelay);
        }
        if (retention > 0) map.put("retention", retention);
        if (maxStorage > 0) map.put("maxStorage", maxStorage);
        if (segmentInterval > 0) map.put("segmentInterval", segmentInterval);
        if (syncCooldown > 0) map.put("syncCooldown", syncCooldown);
        if (waitTimeout > 0) map.put("waitTimeout", waitTimeout);
        if (filesSyncPeriod > 0) map.put("filesSyncPeriod", filesSyncPeriod);
        if (cleanupPeriod > 0) map.put("cleanupPeriod", cleanupPeriod);
        if (blockChainSyncPeriod > 0) map.put("blockChainSyncPeriod", blockChainSyncPeriod);
        if (ioThrottle > 0) map.put("ioThrottle", ioThrottle);
        return map;
    }

    @Override
    public String toString() {
        return "Config{" +
                "syncRelay='" + syncRelay + '\'' +
                ", retention=" + retention +
                ", maxStorage=" + maxStorage +
                ", segmentInterval=" + segmentInterval +
                ", syncCooldown=" + syncCooldown +
                ", waitTimeout=" + waitTimeout +
                ", filesSyncPeriod=" + filesSyncPeriod +
                ", cleanupPeriod=" + cleanupPeriod +
                ", blockChainSyncPeriod=" + blockChainSyncPeriod +
                ", ioThrottle=" + ioThrottle +
                '}';
    }
}
