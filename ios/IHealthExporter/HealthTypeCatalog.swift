import HealthKit

enum HealthTypeCatalog {
    // Names come directly from the iOS 26.2 HealthKit SDK. Unsupported types resolve to nil.
    static let quantityIdentifiers = """
    ActiveEnergyBurned AppleExerciseTime AppleMoveTime AppleSleepingBreathingDisturbances AppleSleepingWristTemperature AppleStandTime AppleWalkingSteadiness AtrialFibrillationBurden BasalBodyTemperature BasalEnergyBurned BloodAlcoholContent BloodGlucose BloodPressureDiastolic BloodPressureSystolic BodyFatPercentage BodyMass BodyMassIndex BodyTemperature CrossCountrySkiingSpeed CyclingCadence CyclingFunctionalThresholdPower CyclingPower CyclingSpeed DietaryBiotin DietaryCaffeine DietaryCalcium DietaryCarbohydrates DietaryChloride DietaryCholesterol DietaryChromium DietaryCopper DietaryEnergyConsumed DietaryFatMonounsaturated DietaryFatPolyunsaturated DietaryFatSaturated DietaryFatTotal DietaryFiber DietaryFolate DietaryIodine DietaryIron DietaryMagnesium DietaryManganese DietaryMolybdenum DietaryNiacin DietaryPantothenicAcid DietaryPhosphorus DietaryPotassium DietaryProtein DietaryRiboflavin DietarySelenium DietarySodium DietarySugar DietaryThiamin DietaryVitaminA DietaryVitaminB12 DietaryVitaminB6 DietaryVitaminC DietaryVitaminD DietaryVitaminE DietaryVitaminK DietaryWater DietaryZinc DistanceCrossCountrySkiing DistanceCycling DistanceDownhillSnowSports DistancePaddleSports DistanceRowing DistanceSkatingSports DistanceSwimming DistanceWalkingRunning DistanceWheelchair ElectrodermalActivity EnvironmentalAudioExposure EnvironmentalSoundReduction EstimatedWorkoutEffortScore FlightsClimbed ForcedExpiratoryVolume1 ForcedVitalCapacity HeadphoneAudioExposure HeartRate HeartRateRecoveryOneMinute HeartRateVariabilitySDNN Height InhalerUsage InsulinDelivery LeanBodyMass NikeFuel NumberOfAlcoholicBeverages NumberOfTimesFallen OxygenSaturation PaddleSportsSpeed PeakExpiratoryFlowRate PeripheralPerfusionIndex PhysicalEffort PushCount RespiratoryRate RestingHeartRate RowingSpeed RunningGroundContactTime RunningPower RunningSpeed RunningStrideLength RunningVerticalOscillation SixMinuteWalkTestDistance StairAscentSpeed StairDescentSpeed StepCount SwimmingStrokeCount TimeInDaylight UVExposure UnderwaterDepth VO2Max WaistCircumference WalkingAsymmetryPercentage WalkingDoubleSupportPercentage WalkingHeartRateAverage WalkingSpeed WalkingStepLength WaterTemperature WorkoutEffortScore
    """.split(separator: " ").map { "HKQuantityTypeIdentifier\($0)" }

    static let categoryIdentifiers = """
    AbdominalCramps Acne AppetiteChanges AppleStandHour AppleWalkingSteadinessEvent AudioExposureEvent BladderIncontinence BleedingAfterPregnancy BleedingDuringPregnancy Bloating BreastPain CervicalMucusQuality ChestTightnessOrPain Chills Constipation Contraceptive Coughing Diarrhea Dizziness DrySkin EnvironmentalAudioExposureEvent Fainting Fatigue Fever GeneralizedBodyAche HairLoss HandwashingEvent Headache HeadphoneAudioExposureEvent Heartburn HighHeartRateEvent HotFlashes HypertensionEvent InfrequentMenstrualCycles IntermenstrualBleeding IrregularHeartRhythmEvent IrregularMenstrualCycles Lactation LossOfSmell LossOfTaste LowCardioFitnessEvent LowHeartRateEvent LowerBackPain MemoryLapse MenstrualFlow MindfulSession MoodChanges Nausea NightSweats OvulationTestResult PelvicPain PersistentIntermenstrualBleeding Pregnancy PregnancyTestResult ProgesteroneTestResult ProlongedMenstrualPeriods RapidPoundingOrFlutteringHeartbeat RunnyNose SexualActivity ShortnessOfBreath SinusCongestion SkippedHeartbeat SleepAnalysis SleepApneaEvent SleepChanges SoreThroat ToothbrushingEvent VaginalDryness Vomiting Wheezing
    """.split(separator: " ").map { "HKCategoryTypeIdentifier\($0)" }

    static var sampleTypes: [HKSampleType] {
        var result: [HKSampleType] = []
        result += quantityIdentifiers.compactMap { HKObjectType.quantityType(forIdentifier: HKQuantityTypeIdentifier(rawValue: $0)) }
        result += categoryIdentifiers.compactMap { HKObjectType.categoryType(forIdentifier: HKCategoryTypeIdentifier(rawValue: $0)) }
        result.append(HKObjectType.workoutType())
        result.append(HKObjectType.electrocardiogramType())
        result.append(HKObjectType.audiogramSampleType())
        result.append(HKObjectType.stateOfMindType())
        result.append(HKObjectType.visionPrescriptionType())
        result.append(HKObjectType.medicationDoseEventType())
        result.append(HKSeriesType.workoutRoute())
        result.append(HKSeriesType.heartbeat())
        if let bloodPressure = HKObjectType.correlationType(forIdentifier: .bloodPressure) { result.append(bloodPressure) }
        if let food = HKObjectType.correlationType(forIdentifier: .food) { result.append(food) }
        if let cda = HKObjectType.documentType(forIdentifier: .CDA) { result.append(cda) }
        result.append(HKScoredAssessmentType(.GAD7))
        result.append(HKScoredAssessmentType(.PHQ9))
        return Dictionary(grouping: result, by: \.identifier).compactMap(\.value.first).sorted { $0.identifier < $1.identifier }
    }
}

enum CanonicalUnit {
    static func forType(_ identifier: String) -> HKUnit? {
        let name = identifier.replacingOccurrences(of: "HKQuantityTypeIdentifier", with: "")
        if name.contains("Energy") || name == "ActiveEnergyBurned" || name == "BasalEnergyBurned" { return .kilocalorie() }
        if name.contains("Temperature") { return .degreeCelsius() }
        if name.contains("Speed") { return .meter().unitDivided(by: .second()) }
        if name.contains("Distance") || name == "Height" || name == "WaistCircumference" || name.contains("StepLength") || name.contains("VerticalOscillation") || name.contains("GroundContactTime") { return name.contains("Time") ? .second() : .meter() }
        if name == "BodyMass" || name == "LeanBodyMass" { return .gramUnit(with: .kilo) }
        if name.contains("Percentage") || name == "OxygenSaturation" || name == "PeripheralPerfusionIndex" || name == "AtrialFibrillationBurden" || name == "BodyFatPercentage" { return .percent() }
        if name.contains("HeartRate") || name == "RespiratoryRate" || name.contains("Cadence") { return .count().unitDivided(by: .minute()) }
        if name.contains("Power") { return .watt() }
        if name.contains("BloodPressure") { return .millimeterOfMercury() }
        if name == "BloodGlucose" { return .gramUnit(with: .milli).unitDivided(by: .literUnit(with: .deci)) }
        if name == "InsulinDelivery" { return .internationalUnit() }
        if name.contains("Volume") || name.contains("VitalCapacity") || name.contains("ExpiratoryFlow") { return .liter() }
        if name.contains("Time") || name == "TimeInDaylight" { return .second() }
        if name == "ElectrodermalActivity" { return .siemenUnit(with: .micro) }
        if name == "BodyMassIndex" || name == "UVExposure" || name.contains("Count") || name.contains("Flights") || name.contains("Usage") || name.contains("Beverages") || name.contains("Fallen") || name == "NikeFuel" { return .count() }
        return nil
    }
}
