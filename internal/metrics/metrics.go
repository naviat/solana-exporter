package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // Node Status Metrics
    NodeHealth     *prometheus.GaugeVec   // Health status (1=healthy, 0=unhealthy)
    NodeVersion    *prometheus.GaugeVec   // Node version info
    
    // Slot/Epoch Metrics
    CurrentSlot    *prometheus.GaugeVec   // Current processed slot
    NetworkSlot    *prometheus.GaugeVec   // Reference node's slot
    SlotBehind     *prometheus.GaugeVec   // How many slots behind reference node
    ConfirmedSlot  *prometheus.GaugeVec   // Confirmed slot height
    SlotHeight     *prometheus.GaugeVec   // Current slot height
    
    // Epoch Details
    EpochInfo      *prometheus.GaugeVec   // Current epoch information
    EpochProgress  *prometheus.GaugeVec   // Epoch progress percentage
    SlotOffset     *prometheus.GaugeVec   // Slot offset within epoch
    SlotsRemaining *prometheus.GaugeVec   // Slots remaining in epoch
    ConfirmedEpochNumber      *prometheus.GaugeVec   // Current epoch number
    ConfirmedEpochFirstSlot   *prometheus.GaugeVec   // First slot of current epoch
    ConfirmedEpochLastSlot    *prometheus.GaugeVec   // Last slot of current epoch
    
    // Block Metrics
    BlockHeight    *prometheus.GaugeVec   // Current block height
    BlockTime      *prometheus.GaugeVec   // Time since last block

    // System Metrics of the Exporter
    SystemMemoryTotal       *prometheus.GaugeVec   // Total system memory
    SystemMemoryUsed        *prometheus.GaugeVec   // Used system memory
    SystemCPUUsage         *prometheus.GaugeVec   // CPU usage percentage
    SystemOpenFDs          *prometheus.GaugeVec   // Open file descriptors
    SystemMaxFDs           *prometheus.GaugeVec   // Maximum file descriptors allowed
    SystemThreads          *prometheus.GaugeVec   // Number of OS threads
    SystemGoroutines       *prometheus.GaugeVec   // Number of goroutines
    SystemGCDuration       *prometheus.SummaryVec // GC duration
    SystemHeapAlloc        *prometheus.GaugeVec   // Heap allocation
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // Node Status Metrics
        NodeHealth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_is_healthy",
                Help: "Is node healthy",
            },
            []string{"endpoint"},
        ),
        NodeVersion: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_version",
                Help: "Node version information",
            },
            []string{"endpoint", "version"},
        ),

        // Slot/Epoch Metrics
        CurrentSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_current_slot",
                Help: "Current processed slot number",
            },
            []string{"endpoint", "commitment"},
        ),
        NetworkSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_network_slot",
                Help: "Network's highest known slot",
            },
            []string{"endpoint"},
        ),
        SlotBehind: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_behind",
                Help: "Number of slots behind the reference node",
            },
            []string{"endpoint"},
        ),
        ConfirmedSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_slot_height",
                Help: "Last confirmed slot height processed",
            },
            []string{"endpoint"},
        ),
        SlotHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_height",
                Help: "Current slot height",
            },
            []string{"endpoint"},
        ),

        // Epoch Details
        EpochInfo: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_epoch",
                Help: "Current epoch information",
            },
            []string{"endpoint"},
        ),
        EpochProgress: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_epoch_progress",
                Help: "Current epoch progress percentage",
            },
            []string{"endpoint"},
        ),
        SlotOffset: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_offset",
                Help: "Slot offset within current epoch",
            },
            []string{"endpoint"},
        ),
        SlotsRemaining: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slots_remaining",
                Help: "Slots remaining in current epoch",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochNumber: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_number",
                Help: "Current epoch",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochFirstSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_first_slot",
                Help: "Current epoch's first slot",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochLastSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_last_slot",
                Help: "Current epoch's last slot",
            },
            []string{"endpoint"},
        ),

        // Block Metrics
        BlockHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_height",
                Help: "Current block height",
            },
            []string{"endpoint"},
        ),
        BlockTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_time",
                Help: "Time since last block in seconds",
            },
            []string{"endpoint"},
        ),

        // System Metrics
        SystemMemoryTotal: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_virtual_memory_bytes",
                Help: "Virtual memory size in bytes",
            },
            []string{"endpoint"},
        ),
        SystemMemoryUsed: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_resident_memory_bytes",
                Help: "Resident memory size in bytes",
            },
            []string{"endpoint"},
        ),
        SystemOpenFDs: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_open_fds",
                Help: "Number of open file descriptors",
            },
            []string{"endpoint"},
        ),
        SystemMaxFDs: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_max_fds",
                Help: "Maximum number of open file descriptors",
            },
            []string{"endpoint"},
        ),
        SystemThreads: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_threads",
                Help: "Number of OS threads created",
            },
            []string{"endpoint"},
        ),
        SystemGoroutines: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_goroutines",
                Help: "Number of goroutines that currently exist",
            },
            []string{"endpoint"},
        ),
        SystemGCDuration: prometheus.NewSummaryVec(
            prometheus.SummaryOpts{
                Name: "go_gc_duration_seconds",
                Help: "A summary of the GC invocation durations",
                Objectives: map[float64]float64{
                    0.5:  0.05,
                    0.9:  0.01,
                    0.99: 0.001,
                },
            },
            []string{"endpoint"},
        ),
        SystemHeapAlloc: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_alloc_bytes",
                Help: "Number of heap bytes allocated and still in use",
            },
            []string{"endpoint"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        m.NodeHealth,
        m.NodeVersion,
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotBehind,
        m.ConfirmedSlot,
        m.SlotHeight,
        m.EpochInfo,
        m.EpochProgress,
        m.SlotOffset,
        m.SlotsRemaining,
        m.ConfirmedEpochNumber,
        m.ConfirmedEpochFirstSlot,
        m.ConfirmedEpochLastSlot,
        m.BlockHeight,
        m.BlockTime,
        m.SystemMemoryTotal,
        m.SystemMemoryUsed,
        m.SystemOpenFDs,
        m.SystemMaxFDs,
        m.SystemThreads,
        m.SystemGoroutines,
        m.SystemGCDuration,
        m.SystemHeapAlloc,
    )

    return m
}
