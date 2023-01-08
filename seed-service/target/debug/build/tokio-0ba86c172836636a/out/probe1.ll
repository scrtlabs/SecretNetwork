; ModuleID = 'probe1.b457ac1c-cgu.0'
source_filename = "probe1.b457ac1c-cgu.0"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128"
target triple = "x86_64-unknown-linux-gnu"

%"core::sync::atomic::AtomicUsize" = type { i64 }
%"std::sys_common::mutex::MovableMutex" = type { %"std::sys::unix::locks::futex::Mutex" }
%"std::sys::unix::locks::futex::Mutex" = type { %"core::sync::atomic::AtomicU32" }
%"core::sync::atomic::AtomicU32" = type { i32 }
%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>" = type { i64, [2 x i64] }
%"std::sync::mutex::Mutex<i32>" = type { %"std::sys_common::mutex::MovableMutex", %"std::sync::poison::Flag", [3 x i8], i32 }
%"std::sync::poison::Flag" = type { %"core::sync::atomic::AtomicBool" }
%"core::sync::atomic::AtomicBool" = type { i8 }
%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err" = type { [1 x i64], { i32*, i8 } }
%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok" = type { [1 x i64], { i32*, i8 } }
%"core::fmt::Arguments" = type { { [0 x { [0 x i8]*, i64 }]*, i64 }, { i64*, i64 }, { [0 x { i8*, i64* }]*, i64 } }
%"core::panic::location::Location" = type { { [0 x i8]*, i64 }, i32, i32 }
%"core::fmt::Formatter" = type { { i64, i64 }, { i64, i64 }, { {}*, [3 x i64]* }, i32, i32, i8, [7 x i8] }
%"core::fmt::builders::DebugStruct" = type { %"core::fmt::Formatter"*, i8, i8, [6 x i8] }
%"unwind::libunwind::_Unwind_Exception" = type { i64, void (i32, %"unwind::libunwind::_Unwind_Exception"*)*, [6 x i64] }
%"unwind::libunwind::_Unwind_Context" = type { [0 x i8] }

@_ZN3std9panicking11panic_count18GLOBAL_PANIC_COUNT17hfc4e5e64a1d87587E = external global %"core::sync::atomic::AtomicUsize"
@alloc36 = private unnamed_addr constant <{ [12 x i8] }> <{ [12 x i8] c"invalid args" }>, align 1
@alloc37 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [12 x i8] }>, <{ [12 x i8] }>* @alloc36, i32 0, i32 0, i32 0), [8 x i8] c"\0C\00\00\00\00\00\00\00" }>, align 8
@alloc34 = private unnamed_addr constant <{}> zeroinitializer, align 8
@alloc73 = private unnamed_addr constant <{ [75 x i8] }> <{ [75 x i8] c"/rustc/4b91a6ea7258a947e59c6522cd5898e7c0a6a88f/library/core/src/fmt/mod.rs" }>, align 1
@alloc74 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [75 x i8] }>, <{ [75 x i8] }>* @alloc73, i32 0, i32 0, i32 0), [16 x i8] c"K\00\00\00\00\00\00\00\87\01\00\00\0D\00\00\00" }>, align 8
@alloc44 = private unnamed_addr constant <{ [49 x i8] }> <{ [49 x i8] c"there is no such thing as an acquire/release load" }>, align 1
@alloc45 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [49 x i8] }>, <{ [49 x i8] }>* @alloc44, i32 0, i32 0, i32 0), [8 x i8] c"1\00\00\00\00\00\00\00" }>, align 8
@alloc87 = private unnamed_addr constant <{ [79 x i8] }> <{ [79 x i8] c"/rustc/4b91a6ea7258a947e59c6522cd5898e7c0a6a88f/library/core/src/sync/atomic.rs" }>, align 1
@alloc76 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00$\0A\00\00\17\00\00\00" }>, align 8
@alloc49 = private unnamed_addr constant <{ [40 x i8] }> <{ [40 x i8] c"there is no such thing as a release load" }>, align 1
@alloc50 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [40 x i8] }>, <{ [40 x i8] }>* @alloc49, i32 0, i32 0, i32 0), [8 x i8] c"(\00\00\00\00\00\00\00" }>, align 8
@alloc78 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00#\0A\00\00\18\00\00\00" }>, align 8
@alloc64 = private unnamed_addr constant <{ [50 x i8] }> <{ [50 x i8] c"there is no such thing as an acquire/release store" }>, align 1
@alloc65 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [50 x i8] }>, <{ [50 x i8] }>* @alloc64, i32 0, i32 0, i32 0), [8 x i8] c"2\00\00\00\00\00\00\00" }>, align 8
@alloc80 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00\16\0A\00\00\17\00\00\00" }>, align 8
@alloc69 = private unnamed_addr constant <{ [42 x i8] }> <{ [42 x i8] c"there is no such thing as an acquire store" }>, align 1
@alloc70 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [42 x i8] }>, <{ [42 x i8] }>* @alloc69, i32 0, i32 0, i32 0), [8 x i8] c"*\00\00\00\00\00\00\00" }>, align 8
@alloc82 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00\15\0A\00\00\18\00\00\00" }>, align 8
@alloc21 = private unnamed_addr constant <{ [60 x i8] }> <{ [60 x i8] c"a failure ordering can't be stronger than a success ordering" }>, align 1
@alloc22 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [60 x i8] }>, <{ [60 x i8] }>* @alloc21, i32 0, i32 0, i32 0), [8 x i8] c"<\00\00\00\00\00\00\00" }>, align 8
@alloc84 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00o\0A\00\00\12\00\00\00" }>, align 8
@alloc26 = private unnamed_addr constant <{ [61 x i8] }> <{ [61 x i8] c"there is no such thing as an acquire/release failure ordering" }>, align 1
@alloc27 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [61 x i8] }>, <{ [61 x i8] }>* @alloc26, i32 0, i32 0, i32 0), [8 x i8] c"=\00\00\00\00\00\00\00" }>, align 8
@alloc86 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00m\0A\00\00\1C\00\00\00" }>, align 8
@alloc31 = private unnamed_addr constant <{ [52 x i8] }> <{ [52 x i8] c"there is no such thing as a release failure ordering" }>, align 1
@alloc32 = private unnamed_addr constant <{ i8*, [8 x i8] }> <{ i8* getelementptr inbounds (<{ [52 x i8] }>, <{ [52 x i8] }>* @alloc31, i32 0, i32 0, i32 0), [8 x i8] c"4\00\00\00\00\00\00\00" }>, align 8
@alloc88 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [79 x i8] }>, <{ [79 x i8] }>* @alloc87, i32 0, i32 0, i32 0), [16 x i8] c"O\00\00\00\00\00\00\00n\0A\00\00\1D\00\00\00" }>, align 8
@alloc106 = private unnamed_addr constant <{ [43 x i8] }> <{ [43 x i8] c"called `Result::unwrap()` on an `Err` value" }>, align 1
@vtable.0 = private unnamed_addr constant <{ i8*, [16 x i8], i8* }> <{ i8* bitcast (void ({ i32*, i8 }*)* @"_ZN4core3ptr98drop_in_place$LT$std..sync..poison..PoisonError$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$$GT$17h9c957925b9119b3bE" to i8*), [16 x i8] c"\10\00\00\00\00\00\00\00\08\00\00\00\00\00\00\00", i8* bitcast (i1 ({ i32*, i8 }*, %"core::fmt::Formatter"*)* @"_ZN76_$LT$std..sync..poison..PoisonError$LT$T$GT$$u20$as$u20$core..fmt..Debug$GT$3fmt17hcff09f99ebe7bed7E" to i8*) }>, align 8
@alloc110 = private unnamed_addr constant <{ [11 x i8] }> <{ [11 x i8] c"PoisonError" }>, align 1
@alloc111 = private unnamed_addr constant <{ [6 x i8] }> <{ [6 x i8] c"<anon>" }>, align 1
@alloc112 = private unnamed_addr constant <{ i8*, [16 x i8] }> <{ i8* getelementptr inbounds (<{ [6 x i8] }>, <{ [6 x i8] }>* @alloc111, i32 0, i32 0, i32 0), [16 x i8] c"\06\00\00\00\00\00\00\00\04\00\00\00\16\00\00\00" }>, align 8
@_ZN6probe15probe8MY_MUTEX17h0dddd4c0d441a112E = internal global <{ [5 x i8], [3 x i8], [4 x i8] }> <{ [5 x i8] zeroinitializer, [3 x i8] undef, [4 x i8] c"\01\00\00\00" }>, align 4

; std::sys_common::mutex::MovableMutex::raw_unlock
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN3std10sys_common5mutex12MovableMutex10raw_unlock17h22cbb10529374e68E(%"std::sys_common::mutex::MovableMutex"* align 4 %self) unnamed_addr #0 {
start:
  %_2 = bitcast %"std::sys_common::mutex::MovableMutex"* %self to %"std::sys::unix::locks::futex::Mutex"*
; call std::sys::unix::locks::futex::Mutex::unlock
  call void @_ZN3std3sys4unix5locks5futex5Mutex6unlock17h3949fabc1e94fb80E(%"std::sys::unix::locks::futex::Mutex"* align 4 %_2)
  br label %bb1

bb1:                                              ; preds = %start
  ret void
}

; std::sys_common::mutex::MovableMutex::raw_lock
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN3std10sys_common5mutex12MovableMutex8raw_lock17he9dc5a1b02aeaecdE(%"std::sys_common::mutex::MovableMutex"* align 4 %self) unnamed_addr #0 {
start:
  %_2 = bitcast %"std::sys_common::mutex::MovableMutex"* %self to %"std::sys::unix::locks::futex::Mutex"*
; call std::sys::unix::locks::futex::Mutex::lock
  call void @_ZN3std3sys4unix5locks5futex5Mutex4lock17hcab9b7d42a119bb3E(%"std::sys::unix::locks::futex::Mutex"* align 4 %_2)
  br label %bb1

bb1:                                              ; preds = %start
  ret void
}

; std::sys::unix::locks::futex::Mutex::lock
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN3std3sys4unix5locks5futex5Mutex4lock17hcab9b7d42a119bb3E(%"std::sys::unix::locks::futex::Mutex"* align 4 %self) unnamed_addr #0 {
start:
  %_7 = alloca i8, align 1
  %_6 = alloca i8, align 1
  %_4 = alloca { i32, i32 }, align 4
  %_5 = bitcast %"std::sys::unix::locks::futex::Mutex"* %self to %"core::sync::atomic::AtomicU32"*
  store i8 2, i8* %_6, align 1
  store i8 0, i8* %_7, align 1
  %0 = load i8, i8* %_6, align 1, !range !2, !noundef !3
  %1 = load i8, i8* %_7, align 1, !range !2, !noundef !3
; call core::sync::atomic::AtomicU32::compare_exchange
  %2 = call { i32, i32 } @_ZN4core4sync6atomic9AtomicU3216compare_exchange17hd0f30cb52f968179E(%"core::sync::atomic::AtomicU32"* align 4 %_5, i32 0, i32 1, i8 %0, i8 %1)
  store { i32, i32 } %2, { i32, i32 }* %_4, align 4
  br label %bb1

bb1:                                              ; preds = %start
; call core::result::Result<T,E>::is_err
  %_2 = call zeroext i1 @"_ZN4core6result19Result$LT$T$C$E$GT$6is_err17h4aab6573248dfafbE"({ i32, i32 }* align 4 %_4)
  br label %bb2

bb2:                                              ; preds = %bb1
  br i1 %_2, label %bb3, label %bb5

bb5:                                              ; preds = %bb4, %bb2
  ret void

bb3:                                              ; preds = %bb2
; call std::sys::unix::locks::futex::Mutex::lock_contended
  call void @_ZN3std3sys4unix5locks5futex5Mutex14lock_contended17hbda10e245d200c70E(%"std::sys::unix::locks::futex::Mutex"* align 4 %self)
  br label %bb4

bb4:                                              ; preds = %bb3
  br label %bb5
}

; std::sys::unix::locks::futex::Mutex::unlock
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN3std3sys4unix5locks5futex5Mutex6unlock17h3949fabc1e94fb80E(%"std::sys::unix::locks::futex::Mutex"* align 4 %self) unnamed_addr #0 {
start:
  %_4 = alloca i8, align 1
  %_3 = bitcast %"std::sys::unix::locks::futex::Mutex"* %self to %"core::sync::atomic::AtomicU32"*
  store i8 1, i8* %_4, align 1
  %0 = load i8, i8* %_4, align 1, !range !2, !noundef !3
; call core::sync::atomic::AtomicU32::swap
  %_2 = call i32 @_ZN4core4sync6atomic9AtomicU324swap17h7fe8a44bc98378caE(%"core::sync::atomic::AtomicU32"* align 4 %_3, i32 0, i8 %0)
  br label %bb1

bb1:                                              ; preds = %start
  %1 = icmp eq i32 %_2, 2
  br i1 %1, label %bb2, label %bb4

bb2:                                              ; preds = %bb1
; call std::sys::unix::locks::futex::Mutex::wake
  call void @_ZN3std3sys4unix5locks5futex5Mutex4wake17hea08090d1f447fbdE(%"std::sys::unix::locks::futex::Mutex"* align 4 %self)
  br label %bb3

bb4:                                              ; preds = %bb1
  br label %bb5

bb5:                                              ; preds = %bb3, %bb4
  ret void

bb3:                                              ; preds = %bb2
  br label %bb5
}

; std::sync::mutex::Mutex<T>::lock
; Function Attrs: nonlazybind uwtable
define void @"_ZN3std4sync5mutex14Mutex$LT$T$GT$4lock17hcf860c54a5d5a141E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %0, %"std::sync::mutex::Mutex<i32>"* align 4 %self) unnamed_addr #1 {
start:
  %_3 = bitcast %"std::sync::mutex::Mutex<i32>"* %self to %"std::sys_common::mutex::MovableMutex"*
; call std::sys_common::mutex::MovableMutex::raw_lock
  call void @_ZN3std10sys_common5mutex12MovableMutex8raw_lock17he9dc5a1b02aeaecdE(%"std::sys_common::mutex::MovableMutex"* align 4 %_3)
  br label %bb1

bb1:                                              ; preds = %start
; call std::sync::mutex::MutexGuard<T>::new
  call void @"_ZN3std4sync5mutex19MutexGuard$LT$T$GT$3new17h9f1622aede1d89d2E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %0, %"std::sync::mutex::Mutex<i32>"* align 4 %self)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret void
}

; std::sync::mutex::MutexGuard<T>::new
; Function Attrs: nonlazybind uwtable
define void @"_ZN3std4sync5mutex19MutexGuard$LT$T$GT$3new17h9f1622aede1d89d2E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %0, %"std::sync::mutex::Mutex<i32>"* align 4 %lock) unnamed_addr #1 {
start:
  %_4 = alloca i32*, align 8
  %_3 = getelementptr inbounds %"std::sync::mutex::Mutex<i32>", %"std::sync::mutex::Mutex<i32>"* %lock, i32 0, i32 1
; call std::sync::poison::Flag::guard
  %1 = call { i8, i8 } @_ZN3std4sync6poison4Flag5guard17h08ae22b22f43727dE(%"std::sync::poison::Flag"* align 1 %_3)
  %2 = extractvalue { i8, i8 } %1, 0
  %_2.0 = trunc i8 %2 to i1
  %_2.1 = extractvalue { i8, i8 } %1, 1
  br label %bb1

bb1:                                              ; preds = %start
  %3 = bitcast i32** %_4 to %"std::sync::mutex::Mutex<i32>"**
  store %"std::sync::mutex::Mutex<i32>"* %lock, %"std::sync::mutex::Mutex<i32>"** %3, align 8
  %4 = load i32*, i32** %_4, align 8, !nonnull !3, !align !4, !noundef !3
; call std::sync::poison::map_result
  call void @_ZN3std4sync6poison10map_result17h959883b67bff79a4E(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %0, i1 zeroext %_2.0, i8 %_2.1, i32* align 4 %4)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret void
}

; std::sync::mutex::MutexGuard<T>::new::{{closure}}
; Function Attrs: inlinehint nonlazybind uwtable
define { i32*, i8 } @"_ZN3std4sync5mutex19MutexGuard$LT$T$GT$3new28_$u7b$$u7b$closure$u7d$$u7d$17hbea9ac1ebb731a71E"(i32* align 4 %_1, i1 zeroext %guard) unnamed_addr #0 {
start:
  %0 = alloca { i32*, i8 }, align 8
  %_5 = bitcast i32* %_1 to %"std::sync::mutex::Mutex<i32>"*
  %1 = bitcast { i32*, i8 }* %0 to %"std::sync::mutex::Mutex<i32>"**
  store %"std::sync::mutex::Mutex<i32>"* %_5, %"std::sync::mutex::Mutex<i32>"** %1, align 8
  %2 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 1
  %3 = zext i1 %guard to i8
  store i8 %3, i8* %2, align 8
  %4 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 0
  %5 = load i32*, i32** %4, align 8, !nonnull !3, !align !4, !noundef !3
  %6 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 1
  %7 = load i8, i8* %6, align 8, !range !5, !noundef !3
  %8 = trunc i8 %7 to i1
  %9 = zext i1 %8 to i8
  %10 = insertvalue { i32*, i8 } undef, i32* %5, 0
  %11 = insertvalue { i32*, i8 } %10, i8 %9, 1
  ret { i32*, i8 } %11
}

; std::sync::poison::map_result
; Function Attrs: nonlazybind uwtable
define void @_ZN3std4sync6poison10map_result17h959883b67bff79a4E(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %0, i1 zeroext %1, i8 %2, i32* align 4 %f) unnamed_addr #1 {
start:
  %_13 = alloca i8, align 1
  %_7 = alloca i8, align 1
  %result = alloca { i8, i8 }, align 1
  %3 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %result, i32 0, i32 0
  %4 = zext i1 %1 to i8
  store i8 %4, i8* %3, align 1
  %5 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %result, i32 0, i32 1
  store i8 %2, i8* %5, align 1
  %6 = bitcast { i8, i8 }* %result to i8*
  %7 = load i8, i8* %6, align 1, !range !5, !noundef !3
  %8 = trunc i8 %7 to i1
  %_3 = zext i1 %8 to i64
  switch i64 %_3, label %bb2 [
    i64 0, label %bb3
    i64 1, label %bb1
  ]

bb2:                                              ; preds = %start
  unreachable

bb3:                                              ; preds = %start
  %9 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %result, i32 0, i32 1
  %10 = load i8, i8* %9, align 1, !range !5, !noundef !3
  %t = trunc i8 %10 to i1
  %11 = zext i1 %t to i8
  store i8 %11, i8* %_7, align 1
  %12 = load i8, i8* %_7, align 1, !range !5, !noundef !3
  %13 = trunc i8 %12 to i1
; call std::sync::mutex::MutexGuard<T>::new::{{closure}}
  %14 = call { i32*, i8 } @"_ZN3std4sync5mutex19MutexGuard$LT$T$GT$3new28_$u7b$$u7b$closure$u7d$$u7d$17hbea9ac1ebb731a71E"(i32* align 4 %f, i1 zeroext %13)
  %_5.0 = extractvalue { i32*, i8 } %14, 0
  %15 = extractvalue { i32*, i8 } %14, 1
  %_5.1 = trunc i8 %15 to i1
  br label %bb4

bb1:                                              ; preds = %start
  %16 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %result, i32 0, i32 1
  %17 = load i8, i8* %16, align 1, !range !5, !noundef !3
  %guard = trunc i8 %17 to i1
  %18 = zext i1 %guard to i8
  store i8 %18, i8* %_13, align 1
  %19 = load i8, i8* %_13, align 1, !range !5, !noundef !3
  %20 = trunc i8 %19 to i1
; call std::sync::mutex::MutexGuard<T>::new::{{closure}}
  %21 = call { i32*, i8 } @"_ZN3std4sync5mutex19MutexGuard$LT$T$GT$3new28_$u7b$$u7b$closure$u7d$$u7d$17hbea9ac1ebb731a71E"(i32* align 4 %f, i1 zeroext %20)
  %_11.0 = extractvalue { i32*, i8 } %21, 0
  %22 = extractvalue { i32*, i8 } %21, 1
  %_11.1 = trunc i8 %22 to i1
  br label %bb5

bb5:                                              ; preds = %bb1
; call std::sync::poison::PoisonError<T>::new
  %23 = call { i32*, i8 } @"_ZN3std4sync6poison20PoisonError$LT$T$GT$3new17ha7586528d3b6c6a6E"(i32* align 4 %_11.0, i1 zeroext %_11.1)
  %_10.0 = extractvalue { i32*, i8 } %23, 0
  %24 = extractvalue { i32*, i8 } %23, 1
  %_10.1 = trunc i8 %24 to i1
  br label %bb6

bb6:                                              ; preds = %bb5
  %25 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %0 to %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err"*
  %26 = getelementptr inbounds %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err", %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err"* %25, i32 0, i32 1
  %27 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %26, i32 0, i32 0
  store i32* %_10.0, i32** %27, align 8
  %28 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %26, i32 0, i32 1
  %29 = zext i1 %_10.1 to i8
  store i8 %29, i8* %28, align 8
  %30 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %0 to i64*
  store i64 1, i64* %30, align 8
  br label %bb7

bb7:                                              ; preds = %bb4, %bb6
  ret void

bb4:                                              ; preds = %bb3
  %31 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %0 to %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok"*
  %32 = getelementptr inbounds %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok", %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok"* %31, i32 0, i32 1
  %33 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %32, i32 0, i32 0
  store i32* %_5.0, i32** %33, align 8
  %34 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %32, i32 0, i32 1
  %35 = zext i1 %_5.1 to i8
  store i8 %35, i8* %34, align 8
  %36 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %0 to i64*
  store i64 0, i64* %36, align 8
  br label %bb7
}

; std::sync::poison::PoisonError<T>::new
; Function Attrs: nonlazybind uwtable
define zeroext i1 @"_ZN3std4sync6poison20PoisonError$LT$T$GT$3new17h7e0956c9731cade0E"(i1 zeroext %guard) unnamed_addr #1 {
start:
  %0 = alloca i8, align 1
  %1 = zext i1 %guard to i8
  store i8 %1, i8* %0, align 1
  %2 = load i8, i8* %0, align 1, !range !5, !noundef !3
  %3 = trunc i8 %2 to i1
  ret i1 %3
}

; std::sync::poison::PoisonError<T>::new
; Function Attrs: nonlazybind uwtable
define { i32*, i8 } @"_ZN3std4sync6poison20PoisonError$LT$T$GT$3new17ha7586528d3b6c6a6E"(i32* align 4 %guard.0, i1 zeroext %guard.1) unnamed_addr #1 {
start:
  %0 = alloca { i32*, i8 }, align 8
  %1 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 0
  store i32* %guard.0, i32** %1, align 8
  %2 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 1
  %3 = zext i1 %guard.1 to i8
  store i8 %3, i8* %2, align 8
  %4 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 0
  %5 = load i32*, i32** %4, align 8, !nonnull !3, !align !4, !noundef !3
  %6 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %0, i32 0, i32 1
  %7 = load i8, i8* %6, align 8, !range !5, !noundef !3
  %8 = trunc i8 %7 to i1
  %9 = zext i1 %8 to i8
  %10 = insertvalue { i32*, i8 } undef, i32* %5, 0
  %11 = insertvalue { i32*, i8 } %10, i8 %9, 1
  ret { i32*, i8 } %11
}

; std::sync::poison::Flag::get
; Function Attrs: inlinehint nonlazybind uwtable
define internal zeroext i1 @_ZN3std4sync6poison4Flag3get17hf64744c57777451fE(%"std::sync::poison::Flag"* align 1 %self) unnamed_addr #0 {
start:
  %_3 = alloca i8, align 1
  %_2 = bitcast %"std::sync::poison::Flag"* %self to %"core::sync::atomic::AtomicBool"*
  store i8 0, i8* %_3, align 1
  %0 = load i8, i8* %_3, align 1, !range !2, !noundef !3
; call core::sync::atomic::AtomicBool::load
  %1 = call zeroext i1 @_ZN4core4sync6atomic10AtomicBool4load17h01d6764f932bcc2fE(%"core::sync::atomic::AtomicBool"* align 1 %_2, i8 %0)
  br label %bb1

bb1:                                              ; preds = %start
  ret i1 %1
}

; std::sync::poison::Flag::done
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN3std4sync6poison4Flag4done17h077a35b5641516f6E(%"std::sync::poison::Flag"* align 1 %self, i8* align 1 %guard) unnamed_addr #0 {
start:
  %_9 = alloca i8, align 1
  %_3 = alloca i8, align 1
  %0 = load i8, i8* %guard, align 1, !range !5, !noundef !3
  %_5 = trunc i8 %0 to i1
  %_4 = xor i1 %_5, true
  br i1 %_4, label %bb2, label %bb1

bb1:                                              ; preds = %start
  store i8 0, i8* %_3, align 1
  br label %bb3

bb2:                                              ; preds = %start
; call std::thread::panicking
  %_6 = call zeroext i1 @_ZN3std6thread9panicking17he8631e8f0128efcaE()
  br label %bb4

bb4:                                              ; preds = %bb2
  %1 = zext i1 %_6 to i8
  store i8 %1, i8* %_3, align 1
  br label %bb3

bb3:                                              ; preds = %bb1, %bb4
  %2 = load i8, i8* %_3, align 1, !range !5, !noundef !3
  %3 = trunc i8 %2 to i1
  br i1 %3, label %bb5, label %bb7

bb7:                                              ; preds = %bb6, %bb3
  ret void

bb5:                                              ; preds = %bb3
  %_8 = bitcast %"std::sync::poison::Flag"* %self to %"core::sync::atomic::AtomicBool"*
  store i8 0, i8* %_9, align 1
  %4 = load i8, i8* %_9, align 1, !range !2, !noundef !3
; call core::sync::atomic::AtomicBool::store
  call void @_ZN4core4sync6atomic10AtomicBool5store17h585508e0bddfcc1dE(%"core::sync::atomic::AtomicBool"* align 1 %_8, i1 zeroext true, i8 %4)
  br label %bb6

bb6:                                              ; preds = %bb5
  br label %bb7
}

; std::sync::poison::Flag::guard
; Function Attrs: inlinehint nonlazybind uwtable
define internal { i8, i8 } @_ZN3std4sync6poison4Flag5guard17h08ae22b22f43727dE(%"std::sync::poison::Flag"* align 1 %self) unnamed_addr #0 {
start:
  %ret = alloca i8, align 1
  %0 = alloca { i8, i8 }, align 1
; call std::thread::panicking
  %_3 = call zeroext i1 @_ZN3std6thread9panicking17he8631e8f0128efcaE()
  br label %bb1

bb1:                                              ; preds = %start
  %1 = zext i1 %_3 to i8
  store i8 %1, i8* %ret, align 1
; call std::sync::poison::Flag::get
  %_4 = call zeroext i1 @_ZN3std4sync6poison4Flag3get17hf64744c57777451fE(%"std::sync::poison::Flag"* align 1 %self)
  br label %bb2

bb2:                                              ; preds = %bb1
  br i1 %_4, label %bb3, label %bb5

bb5:                                              ; preds = %bb2
  %2 = load i8, i8* %ret, align 1, !range !5, !noundef !3
  %_8 = trunc i8 %2 to i1
  %3 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %0, i32 0, i32 1
  %4 = zext i1 %_8 to i8
  store i8 %4, i8* %3, align 1
  %5 = bitcast { i8, i8 }* %0 to i8*
  store i8 0, i8* %5, align 1
  br label %bb6

bb3:                                              ; preds = %bb2
  %6 = load i8, i8* %ret, align 1, !range !5, !noundef !3
  %_7 = trunc i8 %6 to i1
; call std::sync::poison::PoisonError<T>::new
  %_6 = call zeroext i1 @"_ZN3std4sync6poison20PoisonError$LT$T$GT$3new17h7e0956c9731cade0E"(i1 zeroext %_7)
  br label %bb4

bb4:                                              ; preds = %bb3
  %7 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %0, i32 0, i32 1
  %8 = zext i1 %_6 to i8
  store i8 %8, i8* %7, align 1
  %9 = bitcast { i8, i8 }* %0 to i8*
  store i8 1, i8* %9, align 1
  br label %bb6

bb6:                                              ; preds = %bb5, %bb4
  %10 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %0, i32 0, i32 0
  %11 = load i8, i8* %10, align 1, !range !5, !noundef !3
  %12 = trunc i8 %11 to i1
  %13 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %0, i32 0, i32 1
  %14 = load i8, i8* %13, align 1
  %15 = zext i1 %12 to i8
  %16 = insertvalue { i8, i8 } undef, i8 %15, 0
  %17 = insertvalue { i8, i8 } %16, i8 %14, 1
  ret { i8, i8 } %17
}

; std::thread::panicking
; Function Attrs: inlinehint nonlazybind uwtable
define internal zeroext i1 @_ZN3std6thread9panicking17he8631e8f0128efcaE() unnamed_addr #0 {
start:
; call std::panicking::panicking
  %0 = call zeroext i1 @_ZN3std9panicking9panicking17h82ad7f7a330ca650E()
  br label %bb1

bb1:                                              ; preds = %start
  ret i1 %0
}

; std::panicking::panic_count::count_is_zero
; Function Attrs: inlinehint nonlazybind uwtable
define internal zeroext i1 @_ZN3std9panicking11panic_count13count_is_zero17h0022fb635f614f20E() unnamed_addr #0 {
start:
  %_5 = alloca i8, align 1
  %0 = alloca i8, align 1
  store i8 0, i8* %_5, align 1
  %1 = load i8, i8* %_5, align 1, !range !2, !noundef !3
; call core::sync::atomic::AtomicUsize::load
  %_2 = call i64 @_ZN4core4sync6atomic11AtomicUsize4load17h3dd2670e46853084E(%"core::sync::atomic::AtomicUsize"* align 8 @_ZN3std9panicking11panic_count18GLOBAL_PANIC_COUNT17hfc4e5e64a1d87587E, i8 %1)
  br label %bb1

bb1:                                              ; preds = %start
  %_1 = and i64 %_2, 9223372036854775807
  %2 = icmp eq i64 %_1, 0
  br i1 %2, label %bb2, label %bb3

bb2:                                              ; preds = %bb1
  store i8 1, i8* %0, align 1
  br label %bb4

bb3:                                              ; preds = %bb1
; call std::panicking::panic_count::is_zero_slow_path
  %3 = call zeroext i1 @_ZN3std9panicking11panic_count17is_zero_slow_path17h4b45d1e6557870d3E()
  %4 = zext i1 %3 to i8
  store i8 %4, i8* %0, align 1
  br label %bb4

bb4:                                              ; preds = %bb2, %bb3
  %5 = load i8, i8* %0, align 1, !range !5, !noundef !3
  %6 = trunc i8 %5 to i1
  ret i1 %6
}

; std::panicking::panicking
; Function Attrs: inlinehint nonlazybind uwtable
define internal zeroext i1 @_ZN3std9panicking9panicking17h82ad7f7a330ca650E() unnamed_addr #0 {
start:
; call std::panicking::panic_count::count_is_zero
  %_1 = call zeroext i1 @_ZN3std9panicking11panic_count13count_is_zero17h0022fb635f614f20E()
  br label %bb1

bb1:                                              ; preds = %start
  %0 = xor i1 %_1, true
  ret i1 %0
}

; core::fmt::Arguments::new_v1
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %0, [0 x { [0 x i8]*, i64 }]* align 8 %pieces.0, i64 %pieces.1, [0 x { i8*, i64* }]* align 8 %args.0, i64 %args.1) unnamed_addr #0 {
start:
  %_24 = alloca { i64*, i64 }, align 8
  %_16 = alloca %"core::fmt::Arguments", align 8
  %_3 = alloca i8, align 1
  %_4 = icmp ult i64 %pieces.1, %args.1
  br i1 %_4, label %bb1, label %bb2

bb2:                                              ; preds = %start
  %_12 = add i64 %args.1, 1
  %_9 = icmp ugt i64 %pieces.1, %_12
  %1 = zext i1 %_9 to i8
  store i8 %1, i8* %_3, align 1
  br label %bb3

bb1:                                              ; preds = %start
  store i8 1, i8* %_3, align 1
  br label %bb3

bb3:                                              ; preds = %bb2, %bb1
  %2 = load i8, i8* %_3, align 1, !range !5, !noundef !3
  %3 = trunc i8 %2 to i1
  br i1 %3, label %bb4, label %bb6

bb6:                                              ; preds = %bb3
  %4 = bitcast { i64*, i64 }* %_24 to {}**
  store {}* null, {}** %4, align 8
  %5 = bitcast %"core::fmt::Arguments"* %0 to { [0 x { [0 x i8]*, i64 }]*, i64 }*
  %6 = getelementptr inbounds { [0 x { [0 x i8]*, i64 }]*, i64 }, { [0 x { [0 x i8]*, i64 }]*, i64 }* %5, i32 0, i32 0
  store [0 x { [0 x i8]*, i64 }]* %pieces.0, [0 x { [0 x i8]*, i64 }]** %6, align 8
  %7 = getelementptr inbounds { [0 x { [0 x i8]*, i64 }]*, i64 }, { [0 x { [0 x i8]*, i64 }]*, i64 }* %5, i32 0, i32 1
  store i64 %pieces.1, i64* %7, align 8
  %8 = getelementptr inbounds %"core::fmt::Arguments", %"core::fmt::Arguments"* %0, i32 0, i32 1
  %9 = getelementptr inbounds { i64*, i64 }, { i64*, i64 }* %_24, i32 0, i32 0
  %10 = load i64*, i64** %9, align 8, !align !6
  %11 = getelementptr inbounds { i64*, i64 }, { i64*, i64 }* %_24, i32 0, i32 1
  %12 = load i64, i64* %11, align 8
  %13 = getelementptr inbounds { i64*, i64 }, { i64*, i64 }* %8, i32 0, i32 0
  store i64* %10, i64** %13, align 8
  %14 = getelementptr inbounds { i64*, i64 }, { i64*, i64 }* %8, i32 0, i32 1
  store i64 %12, i64* %14, align 8
  %15 = getelementptr inbounds %"core::fmt::Arguments", %"core::fmt::Arguments"* %0, i32 0, i32 2
  %16 = getelementptr inbounds { [0 x { i8*, i64* }]*, i64 }, { [0 x { i8*, i64* }]*, i64 }* %15, i32 0, i32 0
  store [0 x { i8*, i64* }]* %args.0, [0 x { i8*, i64* }]** %16, align 8
  %17 = getelementptr inbounds { [0 x { i8*, i64* }]*, i64 }, { [0 x { i8*, i64* }]*, i64 }* %15, i32 0, i32 1
  store i64 %args.1, i64* %17, align 8
  ret void

bb4:                                              ; preds = %bb3
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_16, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc37 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb5

bb5:                                              ; preds = %bb4
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_16, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc74 to %"core::panic::location::Location"*)) #7
  unreachable
}

; core::ptr::drop_in_place<std::sync::mutex::MutexGuard<i32>>
; Function Attrs: nonlazybind uwtable
define void @"_ZN4core3ptr60drop_in_place$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$17h7807a627274bd8c5E"({ i32*, i8 }* %_1) unnamed_addr #1 {
start:
; call <std::sync::mutex::MutexGuard<T> as core::ops::drop::Drop>::drop
  call void @"_ZN79_$LT$std..sync..mutex..MutexGuard$LT$T$GT$$u20$as$u20$core..ops..drop..Drop$GT$4drop17hb2852f11cf3dd5eeE"({ i32*, i8 }* align 8 %_1)
  br label %bb1

bb1:                                              ; preds = %start
  ret void
}

; core::ptr::drop_in_place<std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>
; Function Attrs: nonlazybind uwtable
define void @"_ZN4core3ptr98drop_in_place$LT$std..sync..poison..PoisonError$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$$GT$17h9c957925b9119b3bE"({ i32*, i8 }* %_1) unnamed_addr #1 {
start:
; call core::ptr::drop_in_place<std::sync::mutex::MutexGuard<i32>>
  call void @"_ZN4core3ptr60drop_in_place$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$17h7807a627274bd8c5E"({ i32*, i8 }* %_1)
  br label %bb1

bb1:                                              ; preds = %start
  ret void
}

; core::sync::atomic::AtomicBool::load
; Function Attrs: inlinehint nonlazybind uwtable
define internal zeroext i1 @_ZN4core4sync6atomic10AtomicBool4load17h01d6764f932bcc2fE(%"core::sync::atomic::AtomicBool"* align 1 %self, i8 %order) unnamed_addr #0 {
start:
  %_6 = bitcast %"core::sync::atomic::AtomicBool"* %self to i8*
  br label %bb1

bb1:                                              ; preds = %start
; call core::sync::atomic::atomic_load
  %_3 = call i8 @_ZN4core4sync6atomic11atomic_load17h4e922ed549b7d2d1E(i8* %_6, i8 %order)
  br label %bb2

bb2:                                              ; preds = %bb1
  %0 = icmp ne i8 %_3, 0
  ret i1 %0
}

; core::sync::atomic::AtomicBool::store
; Function Attrs: inlinehint nonlazybind uwtable
define internal void @_ZN4core4sync6atomic10AtomicBool5store17h585508e0bddfcc1dE(%"core::sync::atomic::AtomicBool"* align 1 %self, i1 zeroext %val, i8 %order) unnamed_addr #0 {
start:
  %_6 = bitcast %"core::sync::atomic::AtomicBool"* %self to i8*
  br label %bb1

bb1:                                              ; preds = %start
  %0 = icmp ule i1 %val, true
  call void @llvm.assume(i1 %0)
  %_7 = zext i1 %val to i8
; call core::sync::atomic::atomic_store
  call void @_ZN4core4sync6atomic12atomic_store17h7e6e6e66b684250aE(i8* %_6, i8 %_7, i8 %order)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret void
}

; core::sync::atomic::AtomicUsize::load
; Function Attrs: inlinehint nonlazybind uwtable
define internal i64 @_ZN4core4sync6atomic11AtomicUsize4load17h3dd2670e46853084E(%"core::sync::atomic::AtomicUsize"* align 8 %self, i8 %order) unnamed_addr #0 {
start:
  %_5 = bitcast %"core::sync::atomic::AtomicUsize"* %self to i64*
  br label %bb1

bb1:                                              ; preds = %start
; call core::sync::atomic::atomic_load
  %0 = call i64 @_ZN4core4sync6atomic11atomic_load17h70be26037c7bb4dfE(i64* %_5, i8 %order)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret i64 %0
}

; core::sync::atomic::atomic_load
; Function Attrs: inlinehint nonlazybind uwtable
define i8 @_ZN4core4sync6atomic11atomic_load17h4e922ed549b7d2d1E(i8* %dst, i8 %0) unnamed_addr #0 {
start:
  %_16 = alloca %"core::fmt::Arguments", align 8
  %_8 = alloca %"core::fmt::Arguments", align 8
  %1 = alloca i8, align 1
  %order = alloca i8, align 1
  store i8 %0, i8* %order, align 1
  %2 = load i8, i8* %order, align 1, !range !2, !noundef !3
  %_3 = zext i8 %2 to i64
  switch i64 %_3, label %bb2 [
    i64 0, label %bb5
    i64 1, label %bb9
    i64 2, label %bb3
    i64 3, label %bb1
    i64 4, label %bb7
  ]

bb2:                                              ; preds = %start
  unreachable

bb5:                                              ; preds = %start
  %3 = load atomic i8, i8* %dst monotonic, align 1
  store i8 %3, i8* %1, align 1
  br label %bb6

bb9:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_8, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc50 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb10

bb3:                                              ; preds = %start
  %4 = load atomic i8, i8* %dst acquire, align 1
  store i8 %4, i8* %1, align 1
  br label %bb4

bb1:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_16, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc45 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb11

bb7:                                              ; preds = %start
  %5 = load atomic i8, i8* %dst seq_cst, align 1
  store i8 %5, i8* %1, align 1
  br label %bb8

bb8:                                              ; preds = %bb7
  br label %bb12

bb12:                                             ; preds = %bb6, %bb4, %bb8
  %6 = load i8, i8* %1, align 1
  ret i8 %6

bb11:                                             ; preds = %bb1
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_16, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc76 to %"core::panic::location::Location"*)) #7
  unreachable

bb4:                                              ; preds = %bb3
  br label %bb12

bb10:                                             ; preds = %bb9
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_8, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc78 to %"core::panic::location::Location"*)) #7
  unreachable

bb6:                                              ; preds = %bb5
  br label %bb12
}

; core::sync::atomic::atomic_load
; Function Attrs: inlinehint nonlazybind uwtable
define i64 @_ZN4core4sync6atomic11atomic_load17h70be26037c7bb4dfE(i64* %dst, i8 %0) unnamed_addr #0 {
start:
  %_16 = alloca %"core::fmt::Arguments", align 8
  %_8 = alloca %"core::fmt::Arguments", align 8
  %1 = alloca i64, align 8
  %order = alloca i8, align 1
  store i8 %0, i8* %order, align 1
  %2 = load i8, i8* %order, align 1, !range !2, !noundef !3
  %_3 = zext i8 %2 to i64
  switch i64 %_3, label %bb2 [
    i64 0, label %bb5
    i64 1, label %bb9
    i64 2, label %bb3
    i64 3, label %bb1
    i64 4, label %bb7
  ]

bb2:                                              ; preds = %start
  unreachable

bb5:                                              ; preds = %start
  %3 = load atomic i64, i64* %dst monotonic, align 8
  store i64 %3, i64* %1, align 8
  br label %bb6

bb9:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_8, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc50 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb10

bb3:                                              ; preds = %start
  %4 = load atomic i64, i64* %dst acquire, align 8
  store i64 %4, i64* %1, align 8
  br label %bb4

bb1:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_16, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc45 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb11

bb7:                                              ; preds = %start
  %5 = load atomic i64, i64* %dst seq_cst, align 8
  store i64 %5, i64* %1, align 8
  br label %bb8

bb8:                                              ; preds = %bb7
  br label %bb12

bb12:                                             ; preds = %bb6, %bb4, %bb8
  %6 = load i64, i64* %1, align 8
  ret i64 %6

bb11:                                             ; preds = %bb1
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_16, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc76 to %"core::panic::location::Location"*)) #7
  unreachable

bb4:                                              ; preds = %bb3
  br label %bb12

bb10:                                             ; preds = %bb9
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_8, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc78 to %"core::panic::location::Location"*)) #7
  unreachable

bb6:                                              ; preds = %bb5
  br label %bb12
}

; core::sync::atomic::atomic_swap
; Function Attrs: inlinehint nonlazybind uwtable
define i32 @_ZN4core4sync6atomic11atomic_swap17hbfbf8869612480faE(i32* %dst, i32 %val, i8 %0) unnamed_addr #0 {
start:
  %1 = alloca i32, align 4
  %order = alloca i8, align 1
  store i8 %0, i8* %order, align 1
  %2 = load i8, i8* %order, align 1, !range !2, !noundef !3
  %_4 = zext i8 %2 to i64
  switch i64 %_4, label %bb2 [
    i64 0, label %bb9
    i64 1, label %bb5
    i64 2, label %bb3
    i64 3, label %bb7
    i64 4, label %bb1
  ]

bb2:                                              ; preds = %start
  unreachable

bb9:                                              ; preds = %start
  %3 = atomicrmw xchg i32* %dst, i32 %val monotonic, align 4
  store i32 %3, i32* %1, align 4
  br label %bb10

bb5:                                              ; preds = %start
  %4 = atomicrmw xchg i32* %dst, i32 %val release, align 4
  store i32 %4, i32* %1, align 4
  br label %bb6

bb3:                                              ; preds = %start
  %5 = atomicrmw xchg i32* %dst, i32 %val acquire, align 4
  store i32 %5, i32* %1, align 4
  br label %bb4

bb7:                                              ; preds = %start
  %6 = atomicrmw xchg i32* %dst, i32 %val acq_rel, align 4
  store i32 %6, i32* %1, align 4
  br label %bb8

bb1:                                              ; preds = %start
  %7 = atomicrmw xchg i32* %dst, i32 %val seq_cst, align 4
  store i32 %7, i32* %1, align 4
  br label %bb11

bb11:                                             ; preds = %bb1
  br label %bb12

bb12:                                             ; preds = %bb10, %bb6, %bb4, %bb8, %bb11
  %8 = load i32, i32* %1, align 4
  ret i32 %8

bb8:                                              ; preds = %bb7
  br label %bb12

bb4:                                              ; preds = %bb3
  br label %bb12

bb6:                                              ; preds = %bb5
  br label %bb12

bb10:                                             ; preds = %bb9
  br label %bb12
}

; core::sync::atomic::atomic_store
; Function Attrs: inlinehint nonlazybind uwtable
define void @_ZN4core4sync6atomic12atomic_store17h7e6e6e66b684250aE(i8* %dst, i8 %val, i8 %0) unnamed_addr #0 {
start:
  %_20 = alloca %"core::fmt::Arguments", align 8
  %_12 = alloca %"core::fmt::Arguments", align 8
  %order = alloca i8, align 1
  store i8 %0, i8* %order, align 1
  %1 = load i8, i8* %order, align 1, !range !2, !noundef !3
  %_4 = zext i8 %1 to i64
  switch i64 %_4, label %bb2 [
    i64 0, label %bb5
    i64 1, label %bb3
    i64 2, label %bb9
    i64 3, label %bb1
    i64 4, label %bb7
  ]

bb2:                                              ; preds = %start
  unreachable

bb5:                                              ; preds = %start
  store atomic i8 %val, i8* %dst monotonic, align 1
  br label %bb6

bb3:                                              ; preds = %start
  store atomic i8 %val, i8* %dst release, align 1
  br label %bb4

bb9:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_12, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc70 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb10

bb1:                                              ; preds = %start
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_20, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc65 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb11

bb7:                                              ; preds = %start
  store atomic i8 %val, i8* %dst seq_cst, align 1
  br label %bb8

bb8:                                              ; preds = %bb7
  br label %bb12

bb12:                                             ; preds = %bb6, %bb4, %bb8
  ret void

bb11:                                             ; preds = %bb1
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_20, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc80 to %"core::panic::location::Location"*)) #7
  unreachable

bb10:                                             ; preds = %bb9
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_12, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc82 to %"core::panic::location::Location"*)) #7
  unreachable

bb4:                                              ; preds = %bb3
  br label %bb12

bb6:                                              ; preds = %bb5
  br label %bb12
}

; core::sync::atomic::atomic_compare_exchange
; Function Attrs: inlinehint nonlazybind uwtable
define { i32, i32 } @_ZN4core4sync6atomic23atomic_compare_exchange17hdbef48ce83569d9fE(i32* %dst, i32 %old, i32 %new, i8 %success, i8 %failure) unnamed_addr #0 {
start:
  %_63 = alloca %"core::fmt::Arguments", align 8
  %_55 = alloca %"core::fmt::Arguments", align 8
  %_47 = alloca %"core::fmt::Arguments", align 8
  %_9 = alloca { i8, i8 }, align 1
  %_8 = alloca { i32, i8 }, align 4
  %0 = alloca { i32, i32 }, align 4
  %1 = bitcast { i8, i8 }* %_9 to i8*
  store i8 %success, i8* %1, align 1
  %2 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  store i8 %failure, i8* %2, align 1
  %3 = bitcast { i8, i8 }* %_9 to i8*
  %4 = load i8, i8* %3, align 1, !range !2, !noundef !3
  %_18 = zext i8 %4 to i64
  switch i64 %_18, label %bb35 [
    i64 0, label %bb1
    i64 1, label %bb3
    i64 2, label %bb4
    i64 3, label %bb5
    i64 4, label %bb6
  ]

bb35:                                             ; preds = %start
  unreachable

bb1:                                              ; preds = %start
  %5 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %6 = load i8, i8* %5, align 1, !range !2, !noundef !3
  %_12 = zext i8 %6 to i64
  %7 = icmp eq i64 %_12, 0
  br i1 %7, label %bb14, label %bb2

bb3:                                              ; preds = %start
  %8 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %9 = load i8, i8* %8, align 1, !range !2, !noundef !3
  %_13 = zext i8 %9 to i64
  %10 = icmp eq i64 %_13, 0
  br i1 %10, label %bb10, label %bb2

bb4:                                              ; preds = %start
  %11 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %12 = load i8, i8* %11, align 1, !range !2, !noundef !3
  %_14 = zext i8 %12 to i64
  switch i64 %_14, label %bb2 [
    i64 0, label %bb18
    i64 2, label %bb8
  ]

bb5:                                              ; preds = %start
  %13 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %14 = load i8, i8* %13, align 1, !range !2, !noundef !3
  %_15 = zext i8 %14 to i64
  switch i64 %_15, label %bb2 [
    i64 0, label %bb20
    i64 2, label %bb12
  ]

bb6:                                              ; preds = %start
  %15 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %16 = load i8, i8* %15, align 1, !range !2, !noundef !3
  %_16 = zext i8 %16 to i64
  switch i64 %_16, label %bb2 [
    i64 0, label %bb22
    i64 2, label %bb24
    i64 4, label %bb16
  ]

bb2:                                              ; preds = %bb1, %bb3, %bb4, %bb5, %bb6
  %17 = getelementptr inbounds { i8, i8 }, { i8, i8 }* %_9, i32 0, i32 1
  %18 = load i8, i8* %17, align 1, !range !2, !noundef !3
  %_17 = zext i8 %18 to i64
  switch i64 %_17, label %bb7 [
    i64 1, label %bb28
    i64 3, label %bb26
  ]

bb22:                                             ; preds = %bb6
  %19 = cmpxchg i32* %dst, i32 %old, i32 %new seq_cst monotonic, align 4
  %20 = extractvalue { i32, i1 } %19, 0
  %21 = extractvalue { i32, i1 } %19, 1
  %22 = zext i1 %21 to i8
  %23 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %20, i32* %23, align 4
  %24 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %22, i8* %24, align 4
  br label %bb23

bb24:                                             ; preds = %bb6
  %25 = cmpxchg i32* %dst, i32 %old, i32 %new seq_cst acquire, align 4
  %26 = extractvalue { i32, i1 } %25, 0
  %27 = extractvalue { i32, i1 } %25, 1
  %28 = zext i1 %27 to i8
  %29 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %26, i32* %29, align 4
  %30 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %28, i8* %30, align 4
  br label %bb25

bb16:                                             ; preds = %bb6
  %31 = cmpxchg i32* %dst, i32 %old, i32 %new seq_cst seq_cst, align 4
  %32 = extractvalue { i32, i1 } %31, 0
  %33 = extractvalue { i32, i1 } %31, 1
  %34 = zext i1 %33 to i8
  %35 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %32, i32* %35, align 4
  %36 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %34, i8* %36, align 4
  br label %bb17

bb17:                                             ; preds = %bb16
  br label %bb31

bb31:                                             ; preds = %bb15, %bb11, %bb19, %bb9, %bb21, %bb13, %bb23, %bb25, %bb17
  %37 = bitcast { i32, i8 }* %_8 to i32*
  %val = load i32, i32* %37, align 4
  %38 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  %39 = load i8, i8* %38, align 4, !range !5, !noundef !3
  %ok = trunc i8 %39 to i1
  br i1 %ok, label %bb32, label %bb33

bb25:                                             ; preds = %bb24
  br label %bb31

bb23:                                             ; preds = %bb22
  br label %bb31

bb20:                                             ; preds = %bb5
  %40 = cmpxchg i32* %dst, i32 %old, i32 %new acq_rel monotonic, align 4
  %41 = extractvalue { i32, i1 } %40, 0
  %42 = extractvalue { i32, i1 } %40, 1
  %43 = zext i1 %42 to i8
  %44 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %41, i32* %44, align 4
  %45 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %43, i8* %45, align 4
  br label %bb21

bb12:                                             ; preds = %bb5
  %46 = cmpxchg i32* %dst, i32 %old, i32 %new acq_rel acquire, align 4
  %47 = extractvalue { i32, i1 } %46, 0
  %48 = extractvalue { i32, i1 } %46, 1
  %49 = zext i1 %48 to i8
  %50 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %47, i32* %50, align 4
  %51 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %49, i8* %51, align 4
  br label %bb13

bb13:                                             ; preds = %bb12
  br label %bb31

bb21:                                             ; preds = %bb20
  br label %bb31

bb18:                                             ; preds = %bb4
  %52 = cmpxchg i32* %dst, i32 %old, i32 %new acquire monotonic, align 4
  %53 = extractvalue { i32, i1 } %52, 0
  %54 = extractvalue { i32, i1 } %52, 1
  %55 = zext i1 %54 to i8
  %56 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %53, i32* %56, align 4
  %57 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %55, i8* %57, align 4
  br label %bb19

bb8:                                              ; preds = %bb4
  %58 = cmpxchg i32* %dst, i32 %old, i32 %new acquire acquire, align 4
  %59 = extractvalue { i32, i1 } %58, 0
  %60 = extractvalue { i32, i1 } %58, 1
  %61 = zext i1 %60 to i8
  %62 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %59, i32* %62, align 4
  %63 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %61, i8* %63, align 4
  br label %bb9

bb9:                                              ; preds = %bb8
  br label %bb31

bb19:                                             ; preds = %bb18
  br label %bb31

bb10:                                             ; preds = %bb3
  %64 = cmpxchg i32* %dst, i32 %old, i32 %new release monotonic, align 4
  %65 = extractvalue { i32, i1 } %64, 0
  %66 = extractvalue { i32, i1 } %64, 1
  %67 = zext i1 %66 to i8
  %68 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %65, i32* %68, align 4
  %69 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %67, i8* %69, align 4
  br label %bb11

bb11:                                             ; preds = %bb10
  br label %bb31

bb14:                                             ; preds = %bb1
  %70 = cmpxchg i32* %dst, i32 %old, i32 %new monotonic monotonic, align 4
  %71 = extractvalue { i32, i1 } %70, 0
  %72 = extractvalue { i32, i1 } %70, 1
  %73 = zext i1 %72 to i8
  %74 = bitcast { i32, i8 }* %_8 to i32*
  store i32 %71, i32* %74, align 4
  %75 = getelementptr inbounds { i32, i8 }, { i32, i8 }* %_8, i32 0, i32 1
  store i8 %73, i8* %75, align 4
  br label %bb15

bb7:                                              ; preds = %bb2
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_63, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc22 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb30

bb28:                                             ; preds = %bb2
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_55, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc32 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb29

bb26:                                             ; preds = %bb2
; call core::fmt::Arguments::new_v1
  call void @_ZN4core3fmt9Arguments6new_v117hd1601d1fda8bde9eE(%"core::fmt::Arguments"* sret(%"core::fmt::Arguments") %_47, [0 x { [0 x i8]*, i64 }]* align 8 bitcast (<{ i8*, [8 x i8] }>* @alloc27 to [0 x { [0 x i8]*, i64 }]*), i64 1, [0 x { i8*, i64* }]* align 8 bitcast (<{}>* @alloc34 to [0 x { i8*, i64* }]*), i64 0)
  br label %bb27

bb30:                                             ; preds = %bb7
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_63, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc84 to %"core::panic::location::Location"*)) #7
  unreachable

bb27:                                             ; preds = %bb26
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_47, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc86 to %"core::panic::location::Location"*)) #7
  unreachable

bb29:                                             ; preds = %bb28
; call core::panicking::panic_fmt
  call void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"* %_55, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc88 to %"core::panic::location::Location"*)) #7
  unreachable

bb15:                                             ; preds = %bb14
  br label %bb31

bb33:                                             ; preds = %bb31
  %76 = getelementptr inbounds { i32, i32 }, { i32, i32 }* %0, i32 0, i32 1
  store i32 %val, i32* %76, align 4
  %77 = bitcast { i32, i32 }* %0 to i32*
  store i32 1, i32* %77, align 4
  br label %bb34

bb32:                                             ; preds = %bb31
  %78 = getelementptr inbounds { i32, i32 }, { i32, i32 }* %0, i32 0, i32 1
  store i32 %val, i32* %78, align 4
  %79 = bitcast { i32, i32 }* %0 to i32*
  store i32 0, i32* %79, align 4
  br label %bb34

bb34:                                             ; preds = %bb33, %bb32
  %80 = getelementptr inbounds { i32, i32 }, { i32, i32 }* %0, i32 0, i32 0
  %81 = load i32, i32* %80, align 4, !range !7, !noundef !3
  %82 = getelementptr inbounds { i32, i32 }, { i32, i32 }* %0, i32 0, i32 1
  %83 = load i32, i32* %82, align 4
  %84 = insertvalue { i32, i32 } undef, i32 %81, 0
  %85 = insertvalue { i32, i32 } %84, i32 %83, 1
  ret { i32, i32 } %85
}

; core::sync::atomic::AtomicU32::compare_exchange
; Function Attrs: inlinehint nonlazybind uwtable
define internal { i32, i32 } @_ZN4core4sync6atomic9AtomicU3216compare_exchange17hd0f30cb52f968179E(%"core::sync::atomic::AtomicU32"* align 4 %self, i32 %current, i32 %new, i8 %success, i8 %failure) unnamed_addr #0 {
start:
  %_7 = bitcast %"core::sync::atomic::AtomicU32"* %self to i32*
  br label %bb1

bb1:                                              ; preds = %start
; call core::sync::atomic::atomic_compare_exchange
  %0 = call { i32, i32 } @_ZN4core4sync6atomic23atomic_compare_exchange17hdbef48ce83569d9fE(i32* %_7, i32 %current, i32 %new, i8 %success, i8 %failure)
  %1 = extractvalue { i32, i32 } %0, 0
  %2 = extractvalue { i32, i32 } %0, 1
  br label %bb2

bb2:                                              ; preds = %bb1
  %3 = insertvalue { i32, i32 } undef, i32 %1, 0
  %4 = insertvalue { i32, i32 } %3, i32 %2, 1
  ret { i32, i32 } %4
}

; core::sync::atomic::AtomicU32::swap
; Function Attrs: inlinehint nonlazybind uwtable
define internal i32 @_ZN4core4sync6atomic9AtomicU324swap17h7fe8a44bc98378caE(%"core::sync::atomic::AtomicU32"* align 4 %self, i32 %val, i8 %order) unnamed_addr #0 {
start:
  %_5 = bitcast %"core::sync::atomic::AtomicU32"* %self to i32*
  br label %bb1

bb1:                                              ; preds = %start
; call core::sync::atomic::atomic_swap
  %0 = call i32 @_ZN4core4sync6atomic11atomic_swap17hbfbf8869612480faE(i32* %_5, i32 %val, i8 %order)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret i32 %0
}

; core::result::Result<T,E>::is_ok
; Function Attrs: inlinehint nonlazybind uwtable
define zeroext i1 @"_ZN4core6result19Result$LT$T$C$E$GT$5is_ok17h0c08ac73895933a6E"({ i32, i32 }* align 4 %self) unnamed_addr #0 {
start:
  %0 = alloca i8, align 1
  %1 = bitcast { i32, i32 }* %self to i32*
  %2 = load i32, i32* %1, align 4, !range !7, !noundef !3
  %_2 = zext i32 %2 to i64
  %3 = icmp eq i64 %_2, 0
  br i1 %3, label %bb2, label %bb1

bb2:                                              ; preds = %start
  store i8 1, i8* %0, align 1
  br label %bb3

bb1:                                              ; preds = %start
  store i8 0, i8* %0, align 1
  br label %bb3

bb3:                                              ; preds = %bb2, %bb1
  %4 = load i8, i8* %0, align 1, !range !5, !noundef !3
  %5 = trunc i8 %4 to i1
  ret i1 %5
}

; core::result::Result<T,E>::is_err
; Function Attrs: inlinehint nonlazybind uwtable
define zeroext i1 @"_ZN4core6result19Result$LT$T$C$E$GT$6is_err17h4aab6573248dfafbE"({ i32, i32 }* align 4 %self) unnamed_addr #0 {
start:
; call core::result::Result<T,E>::is_ok
  %_2 = call zeroext i1 @"_ZN4core6result19Result$LT$T$C$E$GT$5is_ok17h0c08ac73895933a6E"({ i32, i32 }* align 4 %self)
  br label %bb1

bb1:                                              ; preds = %start
  %0 = xor i1 %_2, true
  ret i1 %0
}

; core::result::Result<T,E>::unwrap
; Function Attrs: inlinehint nonlazybind uwtable
define { i32*, i8 } @"_ZN4core6result19Result$LT$T$C$E$GT$6unwrap17hc0a7aa8bb2538917E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %self, %"core::panic::location::Location"* align 8 %0) unnamed_addr #0 personality i32 (i32, i32, i64, %"unwind::libunwind::_Unwind_Exception"*, %"unwind::libunwind::_Unwind_Context"*)* @rust_eh_personality {
start:
  %1 = alloca { i8*, i32 }, align 8
  %e = alloca { i32*, i8 }, align 8
  %2 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %self to i64*
  %_2 = load i64, i64* %2, align 8, !range !8, !noundef !3
  switch i64 %_2, label %bb2 [
    i64 0, label %bb3
    i64 1, label %bb1
  ]

bb2:                                              ; preds = %start
  unreachable

bb3:                                              ; preds = %start
  %3 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %self to %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok"*
  %4 = getelementptr inbounds %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok", %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Ok"* %3, i32 0, i32 1
  %5 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %4, i32 0, i32 0
  %t.0 = load i32*, i32** %5, align 8, !nonnull !3, !align !4, !noundef !3
  %6 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %4, i32 0, i32 1
  %7 = load i8, i8* %6, align 8, !range !5, !noundef !3
  %t.1 = trunc i8 %7 to i1
  %8 = zext i1 %t.1 to i8
  %9 = insertvalue { i32*, i8 } undef, i32* %t.0, 0
  %10 = insertvalue { i32*, i8 } %9, i8 %8, 1
  ret { i32*, i8 } %10

bb1:                                              ; preds = %start
  %11 = bitcast %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %self to %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err"*
  %12 = getelementptr inbounds %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err", %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>::Err"* %11, i32 0, i32 1
  %13 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %12, i32 0, i32 0
  %14 = load i32*, i32** %13, align 8, !nonnull !3, !align !4, !noundef !3
  %15 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %12, i32 0, i32 1
  %16 = load i8, i8* %15, align 8, !range !5, !noundef !3
  %17 = trunc i8 %16 to i1
  %18 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %e, i32 0, i32 0
  store i32* %14, i32** %18, align 8
  %19 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %e, i32 0, i32 1
  %20 = zext i1 %17 to i8
  store i8 %20, i8* %19, align 8
  %_7.0 = bitcast { i32*, i8 }* %e to {}*
; invoke core::result::unwrap_failed
  invoke void @_ZN4core6result13unwrap_failed17hc0baa33ef8bc7db8E([0 x i8]* align 1 bitcast (<{ [43 x i8] }>* @alloc106 to [0 x i8]*), i64 43, {}* align 1 %_7.0, [3 x i64]* align 8 bitcast (<{ i8*, [16 x i8], i8* }>* @vtable.0 to [3 x i64]*), %"core::panic::location::Location"* align 8 %0) #7
          to label %unreachable unwind label %cleanup

bb4:                                              ; preds = %cleanup
; invoke core::ptr::drop_in_place<std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>
  invoke void @"_ZN4core3ptr98drop_in_place$LT$std..sync..poison..PoisonError$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$$GT$17h9c957925b9119b3bE"({ i32*, i8 }* %e) #8
          to label %bb5 unwind label %abort

cleanup:                                          ; preds = %bb1
  %21 = landingpad { i8*, i32 }
          cleanup
  %22 = extractvalue { i8*, i32 } %21, 0
  %23 = extractvalue { i8*, i32 } %21, 1
  %24 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %1, i32 0, i32 0
  store i8* %22, i8** %24, align 8
  %25 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %1, i32 0, i32 1
  store i32 %23, i32* %25, align 8
  br label %bb4

unreachable:                                      ; preds = %bb1
  unreachable

abort:                                            ; preds = %bb4
  %26 = landingpad { i8*, i32 }
          cleanup
; call core::panicking::panic_no_unwind
  call void @_ZN4core9panicking15panic_no_unwind17hd9b8f600c78bc0e5E() #9
  unreachable

bb5:                                              ; preds = %bb4
  %27 = bitcast { i8*, i32 }* %1 to i8**
  %28 = load i8*, i8** %27, align 8
  %29 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %1, i32 0, i32 1
  %30 = load i32, i32* %29, align 8
  %31 = insertvalue { i8*, i32 } undef, i8* %28, 0
  %32 = insertvalue { i8*, i32 } %31, i32 %30, 1
  resume { i8*, i32 } %32
}

; <std::sync::poison::PoisonError<T> as core::fmt::Debug>::fmt
; Function Attrs: nonlazybind uwtable
define zeroext i1 @"_ZN76_$LT$std..sync..poison..PoisonError$LT$T$GT$$u20$as$u20$core..fmt..Debug$GT$3fmt17hcff09f99ebe7bed7E"({ i32*, i8 }* align 8 %self, %"core::fmt::Formatter"* align 8 %f) unnamed_addr #1 {
start:
  %_4 = alloca %"core::fmt::builders::DebugStruct", align 8
; call core::fmt::Formatter::debug_struct
  call void @_ZN4core3fmt9Formatter12debug_struct17h6b467d725b90e1a0E(%"core::fmt::builders::DebugStruct"* sret(%"core::fmt::builders::DebugStruct") %_4, %"core::fmt::Formatter"* align 8 %f, [0 x i8]* align 1 bitcast (<{ [11 x i8] }>* @alloc110 to [0 x i8]*), i64 11)
  br label %bb1

bb1:                                              ; preds = %start
; call core::fmt::builders::DebugStruct::finish_non_exhaustive
  %0 = call zeroext i1 @_ZN4core3fmt8builders11DebugStruct21finish_non_exhaustive17h270111d87be47c7aE(%"core::fmt::builders::DebugStruct"* align 8 %_4)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret i1 %0
}

; <std::sync::mutex::MutexGuard<T> as core::ops::drop::Drop>::drop
; Function Attrs: inlinehint nonlazybind uwtable
define void @"_ZN79_$LT$std..sync..mutex..MutexGuard$LT$T$GT$$u20$as$u20$core..ops..drop..Drop$GT$4drop17hb2852f11cf3dd5eeE"({ i32*, i8 }* align 8 %self) unnamed_addr #0 {
start:
  %0 = bitcast { i32*, i8 }* %self to %"std::sync::mutex::Mutex<i32>"**
  %_8 = load %"std::sync::mutex::Mutex<i32>"*, %"std::sync::mutex::Mutex<i32>"** %0, align 8, !nonnull !3, !align !4, !noundef !3
  %_3 = getelementptr inbounds %"std::sync::mutex::Mutex<i32>", %"std::sync::mutex::Mutex<i32>"* %_8, i32 0, i32 1
  %_5 = getelementptr inbounds { i32*, i8 }, { i32*, i8 }* %self, i32 0, i32 1
; call std::sync::poison::Flag::done
  call void @_ZN3std4sync6poison4Flag4done17h077a35b5641516f6E(%"std::sync::poison::Flag"* align 1 %_3, i8* align 1 %_5)
  br label %bb1

bb1:                                              ; preds = %start
  %1 = bitcast { i32*, i8 }* %self to %"std::sync::mutex::Mutex<i32>"**
  %_9 = load %"std::sync::mutex::Mutex<i32>"*, %"std::sync::mutex::Mutex<i32>"** %1, align 8, !nonnull !3, !align !4, !noundef !3
  %_7 = bitcast %"std::sync::mutex::Mutex<i32>"* %_9 to %"std::sys_common::mutex::MovableMutex"*
; call std::sys_common::mutex::MovableMutex::raw_unlock
  call void @_ZN3std10sys_common5mutex12MovableMutex10raw_unlock17h22cbb10529374e68E(%"std::sys_common::mutex::MovableMutex"* align 4 %_7)
  br label %bb2

bb2:                                              ; preds = %bb1
  ret void
}

; <std::sync::mutex::MutexGuard<T> as core::ops::deref::Deref>::deref
; Function Attrs: nonlazybind uwtable
define align 4 i32* @"_ZN81_$LT$std..sync..mutex..MutexGuard$LT$T$GT$$u20$as$u20$core..ops..deref..Deref$GT$5deref17h0a69e056a78af93cE"({ i32*, i8 }* align 8 %self) unnamed_addr #1 {
start:
  %0 = bitcast { i32*, i8 }* %self to %"std::sync::mutex::Mutex<i32>"**
  %_4 = load %"std::sync::mutex::Mutex<i32>"*, %"std::sync::mutex::Mutex<i32>"** %0, align 8, !nonnull !3, !align !4, !noundef !3
  %_3 = getelementptr inbounds %"std::sync::mutex::Mutex<i32>", %"std::sync::mutex::Mutex<i32>"* %_4, i32 0, i32 3
  br label %bb1

bb1:                                              ; preds = %start
  ret i32* %_3
}

; probe1::probe
; Function Attrs: nonlazybind uwtable
define void @_ZN6probe15probe17he098e2c9f54edf27E() unnamed_addr #1 personality i32 (i32, i32, i64, %"unwind::libunwind::_Unwind_Exception"*, %"unwind::libunwind::_Unwind_Context"*)* @rust_eh_personality {
start:
  %0 = alloca { i8*, i32 }, align 8
  %_4 = alloca %"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>", align 8
  %_3 = alloca { i32*, i8 }, align 8
; call std::sync::mutex::Mutex<T>::lock
  call void @"_ZN3std4sync5mutex14Mutex$LT$T$GT$4lock17hcf860c54a5d5a141E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* sret(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>") %_4, %"std::sync::mutex::Mutex<i32>"* align 4 bitcast (<{ [5 x i8], [3 x i8], [4 x i8] }>* @_ZN6probe15probe8MY_MUTEX17h0dddd4c0d441a112E to %"std::sync::mutex::Mutex<i32>"*))
  br label %bb1

bb1:                                              ; preds = %start
; call core::result::Result<T,E>::unwrap
  %1 = call { i32*, i8 } @"_ZN4core6result19Result$LT$T$C$E$GT$6unwrap17hc0a7aa8bb2538917E"(%"core::result::Result<std::sync::mutex::MutexGuard<i32>, std::sync::poison::PoisonError<std::sync::mutex::MutexGuard<i32>>>"* %_4, %"core::panic::location::Location"* align 8 bitcast (<{ i8*, [16 x i8] }>* @alloc112 to %"core::panic::location::Location"*))
  store { i32*, i8 } %1, { i32*, i8 }* %_3, align 8
  br label %bb2

bb2:                                              ; preds = %bb1
; invoke <std::sync::mutex::MutexGuard<T> as core::ops::deref::Deref>::deref
  %_1 = invoke align 4 i32* @"_ZN81_$LT$std..sync..mutex..MutexGuard$LT$T$GT$$u20$as$u20$core..ops..deref..Deref$GT$5deref17h0a69e056a78af93cE"({ i32*, i8 }* align 8 %_3)
          to label %bb3 unwind label %cleanup

bb5:                                              ; preds = %cleanup
; invoke core::ptr::drop_in_place<std::sync::mutex::MutexGuard<i32>>
  invoke void @"_ZN4core3ptr60drop_in_place$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$17h7807a627274bd8c5E"({ i32*, i8 }* %_3) #8
          to label %bb6 unwind label %abort

cleanup:                                          ; preds = %bb2
  %2 = landingpad { i8*, i32 }
          cleanup
  %3 = extractvalue { i8*, i32 } %2, 0
  %4 = extractvalue { i8*, i32 } %2, 1
  %5 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %0, i32 0, i32 0
  store i8* %3, i8** %5, align 8
  %6 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %0, i32 0, i32 1
  store i32 %4, i32* %6, align 8
  br label %bb5

bb3:                                              ; preds = %bb2
; call core::ptr::drop_in_place<std::sync::mutex::MutexGuard<i32>>
  call void @"_ZN4core3ptr60drop_in_place$LT$std..sync..mutex..MutexGuard$LT$i32$GT$$GT$17h7807a627274bd8c5E"({ i32*, i8 }* %_3)
  br label %bb4

abort:                                            ; preds = %bb5
  %7 = landingpad { i8*, i32 }
          cleanup
; call core::panicking::panic_no_unwind
  call void @_ZN4core9panicking15panic_no_unwind17hd9b8f600c78bc0e5E() #9
  unreachable

bb6:                                              ; preds = %bb5
  %8 = bitcast { i8*, i32 }* %0 to i8**
  %9 = load i8*, i8** %8, align 8
  %10 = getelementptr inbounds { i8*, i32 }, { i8*, i32 }* %0, i32 0, i32 1
  %11 = load i32, i32* %10, align 8
  %12 = insertvalue { i8*, i32 } undef, i8* %9, 0
  %13 = insertvalue { i8*, i32 } %12, i32 %11, 1
  resume { i8*, i32 } %13

bb4:                                              ; preds = %bb3
  ret void
}

; std::sys::unix::locks::futex::Mutex::lock_contended
; Function Attrs: cold nonlazybind uwtable
declare void @_ZN3std3sys4unix5locks5futex5Mutex14lock_contended17hbda10e245d200c70E(%"std::sys::unix::locks::futex::Mutex"* align 4) unnamed_addr #2

; std::sys::unix::locks::futex::Mutex::wake
; Function Attrs: cold nonlazybind uwtable
declare void @_ZN3std3sys4unix5locks5futex5Mutex4wake17hea08090d1f447fbdE(%"std::sys::unix::locks::futex::Mutex"* align 4) unnamed_addr #2

; std::panicking::panic_count::is_zero_slow_path
; Function Attrs: cold noinline nonlazybind uwtable
declare zeroext i1 @_ZN3std9panicking11panic_count17is_zero_slow_path17h4b45d1e6557870d3E() unnamed_addr #3

; core::panicking::panic_fmt
; Function Attrs: cold noinline noreturn nonlazybind uwtable
declare void @_ZN4core9panicking9panic_fmt17h1de71520faaa17d3E(%"core::fmt::Arguments"*, %"core::panic::location::Location"* align 8) unnamed_addr #4

; Function Attrs: inaccessiblememonly nofree nosync nounwind willreturn
declare void @llvm.assume(i1 noundef) #5

; Function Attrs: nonlazybind uwtable
declare i32 @rust_eh_personality(i32, i32, i64, %"unwind::libunwind::_Unwind_Exception"*, %"unwind::libunwind::_Unwind_Context"*) unnamed_addr #1

; core::result::unwrap_failed
; Function Attrs: cold noinline noreturn nonlazybind uwtable
declare void @_ZN4core6result13unwrap_failed17hc0baa33ef8bc7db8E([0 x i8]* align 1, i64, {}* align 1, [3 x i64]* align 8, %"core::panic::location::Location"* align 8) unnamed_addr #4

; core::panicking::panic_no_unwind
; Function Attrs: cold noinline noreturn nounwind nonlazybind uwtable
declare void @_ZN4core9panicking15panic_no_unwind17hd9b8f600c78bc0e5E() unnamed_addr #6

; core::fmt::Formatter::debug_struct
; Function Attrs: nonlazybind uwtable
declare void @_ZN4core3fmt9Formatter12debug_struct17h6b467d725b90e1a0E(%"core::fmt::builders::DebugStruct"* sret(%"core::fmt::builders::DebugStruct"), %"core::fmt::Formatter"* align 8, [0 x i8]* align 1, i64) unnamed_addr #1

; core::fmt::builders::DebugStruct::finish_non_exhaustive
; Function Attrs: nonlazybind uwtable
declare zeroext i1 @_ZN4core3fmt8builders11DebugStruct21finish_non_exhaustive17h270111d87be47c7aE(%"core::fmt::builders::DebugStruct"* align 8) unnamed_addr #1

attributes #0 = { inlinehint nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #1 = { nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #2 = { cold nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #3 = { cold noinline nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #4 = { cold noinline noreturn nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #5 = { inaccessiblememonly nofree nosync nounwind willreturn }
attributes #6 = { cold noinline noreturn nounwind nonlazybind uwtable "probe-stack"="__rust_probestack" "target-cpu"="x86-64" }
attributes #7 = { noreturn }
attributes #8 = { noinline }
attributes #9 = { noinline noreturn nounwind }

!llvm.module.flags = !{!0, !1}

!0 = !{i32 7, !"PIC Level", i32 2}
!1 = !{i32 2, !"RtLibUseGOT", i32 1}
!2 = !{i8 0, i8 5}
!3 = !{}
!4 = !{i64 4}
!5 = !{i8 0, i8 2}
!6 = !{i64 8}
!7 = !{i32 0, i32 2}
!8 = !{i64 0, i64 2}
